package main

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ettle/strcase"
	"github.com/go-acme/lego/v4/lego"
	"github.com/ldez/traefik-certs-cleaner/internal/traefik"
	"github.com/urfave/cli/v2"
)

const (
	flagSrcFile      = "src"
	flagDstFile      = "dst"
	flagDomain       = "domain"
	flagResolverName = "resolver-name"
	flagRevoke       = "revoke"
	flagDryRun       = "dry-run"
)

type configuration struct {
	Source       string
	Destination  string
	Domain       string
	ResolverName string
	Revoke       bool
	DryRun       bool
}

func newConfiguration(cliCtx *cli.Context) configuration {
	return configuration{
		Source:       cliCtx.Path(flagSrcFile),
		Destination:  cliCtx.Path(flagDstFile),
		Domain:       cliCtx.String(flagDomain),
		ResolverName: cliCtx.String(flagResolverName),
		Revoke:       cliCtx.Bool(flagRevoke),
		DryRun:       cliCtx.Bool(flagDryRun),
	}
}

func main() {
	app := &cli.App{
		Name:        "traefik-certs-cleaner",
		Description: "Clean ACME certificates from Traefik acme.json file." + helpMessage(),
		Usage:       "Traefik Certificates Cleaner",
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:    flagSrcFile,
				Aliases: []string{"s"},
				Usage:   "Path to the acme.json file.",
				EnvVars: []string{strcase.ToSNAKE(flagSrcFile)},
				Value:   "./acme.json",
			},
			&cli.PathFlag{
				Name:    flagDstFile,
				Aliases: []string{"o"},
				Usage:   "Path to the output of the acme.json file.",
				EnvVars: []string{strcase.ToSNAKE(flagDstFile)},
				Value:   "./acme-new.json",
			},
			&cli.StringFlag{
				Name:    flagResolverName,
				Aliases: []string{"r"},
				Usage:   "Name of the resolver. Use * to handle all resolvers.",
				EnvVars: []string{strcase.ToSNAKE(flagResolverName)},
				Value:   "*",
			},
			&cli.StringFlag{
				Name:    flagDomain,
				Aliases: []string{"d"},
				Usage:   "Domains to remove. Use * to remove all certificates.",
				EnvVars: []string{strcase.ToSNAKE(flagDomain)},
				Value:   "*",
			},
			&cli.BoolFlag{
				Name:    flagRevoke,
				Usage:   "Revoke certificates",
				EnvVars: []string{strcase.ToSNAKE(flagRevoke)},
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    flagDryRun,
				Usage:   "Dry run mode.",
				EnvVars: []string{strcase.ToSNAKE(flagDryRun)},
				Value:   true,
			},
		},
		Action: func(cliCtx *cli.Context) error {
			return cleaner{configuration: newConfiguration(cliCtx)}.run()
		},
		After: func(_ *cli.Context) error {
			help()
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal("Error while executing command ", err)
	}
}

type cleaner struct {
	configuration
}

func (c cleaner) run() error {
	data := map[string]*traefik.StoredData{}
	err := readJSONFile(c.Source, &data)
	if err != nil {
		return err
	}

	err = c.clean(c.configuration, data)
	if err != nil {
		return err
	}

	var encoder *json.Encoder
	if c.DryRun {
		encoder = json.NewEncoder(os.Stdout)
	} else {
		output, err := os.Create(c.Destination)
		if err != nil {
			return err
		}
		defer func() { _ = output.Close() }()

		encoder = json.NewEncoder(output)
	}

	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (c cleaner) clean(config configuration, data map[string]*traefik.StoredData) error {
	for rName, storedData := range data {
		if config.ResolverName != "*" && config.ResolverName != rName {
			continue
		}

		if config.Domain == "*" {
			c.revoke(storedData.Account, storedData.Certificates)
			storedData.Certificates = make([]*traefik.CertAndStore, 0)
			continue
		}

		var keep []*traefik.CertAndStore
		var toRevoke []*traefik.CertAndStore

		for _, cert := range storedData.Certificates {
			if strings.HasSuffix(cert.Domain.Main, config.Domain) || containsSuffixes(cert.Domain.SANs, config.Domain) {
				toRevoke = append(toRevoke, cert)
				continue
			}
			if strings.HasSuffix(cert.Certificate.Domain.Main, config.Domain) || containsSuffixes(cert.Certificate.Domain.SANs, config.Domain) {
				toRevoke = append(toRevoke, cert)
				continue
			}

			certificate, err := getX509Certificate(&cert.Certificate)
			if err != nil {
				return err
			}

			if strings.HasSuffix(certificate.Subject.CommonName, config.Domain) || containsSuffixes(certificate.DNSNames, config.Domain) {
				toRevoke = append(toRevoke, cert)
				continue
			}

			keep = append(keep, cert)
		}

		storedData.Certificates = keep

		c.revoke(storedData.Account, toRevoke)
	}

	return nil
}

func (c cleaner) revoke(account *traefik.Account, certificates []*traefik.CertAndStore) {
	if !c.Revoke {
		return
	}

	if !c.DryRun {
		log.Println("Revoke certificate")
		return
	}

	config := lego.NewConfig(account)
	config.CADirURL = lego.LEDirectoryProduction
	config.UserAgent = "ldez-traefik-certs-cleaner"

	client, err := lego.NewClient(config)
	if err != nil {
		log.Fatalf("Could not create client: %v", err)
	}

	for _, certificate := range certificates {
		err := client.Certificate.Revoke(certificate.Certificate.Certificate)
		if err != nil {
			log.Printf("Failed to revoke certificate for %s: %v", certificate.Domain, err)
		}
	}
}

func containsSuffixes(domains []string, suffix string) bool {
	for _, domain := range domains {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	return false
}

func getX509Certificate(cert *traefik.Certificate) (*x509.Certificate, error) {
	tlsCert, err := tls.X509KeyPair(cert.Certificate, cert.Key)
	if err != nil {
		return nil, err
	}

	crt := tlsCert.Leaf
	if crt == nil {
		crt, err = x509.ParseCertificate(tlsCert.Certificate[0])
		if err != nil {
			return nil, err
		}
	}

	return crt, err
}

func readJSONFile(acmeFile string, data interface{}) error {
	source, err := os.Open(filepath.Clean(acmeFile))
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", acmeFile, err)
	}
	defer func() { _ = source.Close() }()

	err = json.NewDecoder(source).Decode(data)
	if errors.Is(err, io.EOF) {
		log.Printf("warn: file %q may not be ready: %v", acmeFile, err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to unmarshal file %q: %w", acmeFile, err)
	}

	return nil
}

func help() {
	var maxInt int64 = 10 // -> 10%
	if time.Now().Month() == time.December {
		maxInt = 2 // -> 50%
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(maxInt))
	if n.Cmp(big.NewInt(0)) != 0 {
		return
	}

	log.SetFlags(0)

	log.Println(helpMessage())
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func helpMessage() string {
	pStyle := lipgloss.NewStyle().
		Padding(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("161")).
		Align(lipgloss.Center)

	hStyle := lipgloss.NewStyle().Bold(true)

	s := fmt.Sprintln(hStyle.Render("Request for Donation."))
	s += `
I need your help!
Donations fund the maintenance and development of traefik-certs-cleaner.
Click on this link to donate: https://donate.ldez.dev`

	return "\n" + pStyle.Render(s)
}
