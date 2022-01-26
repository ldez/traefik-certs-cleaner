# traefik-certs-cleaner

[![GitHub release](https://img.shields.io/github/release/ldez/traefik-certs-cleaner.svg)](https://github.com/ldez/traefik-certs-cleaner/releases/latest)
[![Build Status](https://github.com/ldez/traefik-certs-cleaner/workflows/Main/badge.svg?branch=master)](https://github.com/ldez/traefik-certs-cleaner/actions)
[![Docker Image Version (latest semver)](https://img.shields.io/docker/v/ldez/traefik-certs-cleaner)](https://hub.docker.com/r/ldez/traefik-certs-cleaner/)
[![Go Report Card](https://goreportcard.com/badge/github.com/ldez/traefik-certs-cleaner)](https://goreportcard.com/report/github.com/ldez/traefik-certs-cleaner)

If you appreciate this project:

[![Sponsor](https://img.shields.io/badge/Sponsor%20me-%E2%9D%A4%EF%B8%8F-pink)](https://github.com/sponsors/ldez)

## Description

traefik-certs-cleaner is a simple helper to clean acme.json file.

It creates a new acme.json file (`acme-new.json` by default) without the certificates that you want to remove.
After the cleaning, you should replace the content of the original `acme.json` by the content of the new file.
Then you have to restart your Traefik instance.

```
NAME:
   traefik-certs-cleaner - Traefik Certificates Cleaner

USAGE:
   traefik-certs-cleaner [global options] command [command options] [arguments...]

DESCRIPTION:
   Clean ACME certificates from Traefik acme.json file.

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --src value, -s value            Path to the acme.json file. (default: "./acme.json") [$SRC]
   --dst value, -o value            Path to the output of the acme.json file. (default: "./acme-new.json") [$DST]
   --resolver-name value, -r value  Name of the resolver. Use * to handle all resolvers. (default: "*") [$RESOLVER_NAME]
   --domain value, -d value         Domains to remove. Use * to remove all certificates. (default: "*") [$DOMAIN]
   --dry-run                        Dry run mode. (default: true) [$DRY_RUN]
   --help, -h                       show help (default: false)
```

## Examples

### Dry run (Default)

```console
$ traefik-certs-cleaner --src=./acme.json
```

The content of the new file is displayed to the console output.

### Remove all certificates

```console
$ traefik-certs-cleaner --src=./acme.json --dry-run=false
```

Creates a new file `./acme-new.json`.

### Remove all certificates for a Specific Resolver

```console
$ traefik-certs-cleaner --src=./acme.json --resolver-name=myresolver --dry-run=false
```

Creates a new file `./acme-new.json`.

### Remove the certificates of a Specific Domain

```console
$ traefik-certs-cleaner --src=./acme.json --domain=example.com --dry-run=false
```

Creates a new file `./acme-new.json`.

