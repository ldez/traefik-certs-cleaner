package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ettle/strcase"
	"github.com/traefik/traefik/v2/pkg/provider/acme"
	"github.com/urfave/cli/v2"
)

const (
	flagSrcFile      = "src"
	flagDstFile      = "dst"
	flagDomain       = "domain"
	flagResolverName = "resolver-name"
	flagDryRun       = "dry-run"
)

func main() {
	app := &cli.App{
		Name:        "traefik-certs-cleaner",
		Description: "Clean ACME certificates from Traefik acme.json file.",
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
				Name:    flagDryRun,
				Usage:   "Dry run mode.",
				EnvVars: []string{strcase.ToSNAKE(flagDryRun)},
				Value:   true,
			},
		},
		Action: run,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal("Error while executing command ", err)
	}
}

func run(cliCtx *cli.Context) error {
	data := map[string]*acme.StoredData{}
	err := readJSONFile(cliCtx.Path(flagSrcFile), &data)
	if err != nil {
		return err
	}

	target := cliCtx.String(flagDomain)

	for rName, storedData := range data {
		if cliCtx.String(flagResolverName) != "*" && cliCtx.String(flagResolverName) != rName {
			continue
		}

		if target == "*" {
			storedData.Certificates = make([]*acme.CertAndStore, 0)
			continue
		}

		var keep []*acme.CertAndStore
		for _, cert := range storedData.Certificates {
			if strings.HasSuffix(cert.Domain.Main, target) || containsSuffixes(cert.Domain.SANs, target) {
				continue
			}
			if strings.HasSuffix(cert.Certificate.Domain.Main, target) || containsSuffixes(cert.Certificate.Domain.SANs, target) {
				continue
			}

			certificate, err := getX509Certificate(&cert.Certificate)
			if err != nil {
				return err
			}

			if strings.HasSuffix(certificate.Subject.CommonName, target) || containsSuffixes(certificate.DNSNames, target) {
				continue
			}

			keep = append(keep, cert)
		}

		storedData.Certificates = keep
	}

	var encoder *json.Encoder
	if cliCtx.Bool(flagDryRun) {
		encoder = json.NewEncoder(os.Stdout)
	} else {
		output, err := os.Create(cliCtx.Path(flagDstFile))
		if err != nil {
			return err
		}
		defer func() { _ = output.Close() }()

		encoder = json.NewEncoder(output)
	}

	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func containsSuffixes(domains []string, suffix string) bool {
	for _, domain := range domains {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	return false
}

func getX509Certificate(cert *acme.Certificate) (*x509.Certificate, error) {
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
