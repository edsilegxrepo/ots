module github.com/edsilegxrepo/ots/cmd/ots-cli

go 1.26.0

toolchain go1.26.5

replace (
	github.com/edsilegxrepo/ots => ../..
	github.com/edsilegxrepo/ots/pkg/client => ../../pkg/client
	github.com/edsilegxrepo/ots/pkg/customization => ../../pkg/customization
)

require (
	github.com/edsilegxrepo/ots v1.21.9
	github.com/edsilegxrepo/ots/pkg/client v0.0.0-20260806231054-72d901945284
	github.com/sirupsen/logrus v1.9.4
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/edsilegxrepo/ots/pkg/customization v0.0.0-20260806231054-72d901945284 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
