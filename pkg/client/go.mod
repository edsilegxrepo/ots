module github.com/Luzifer/ots/pkg/client

go 1.25.0

toolchain go1.26.5

replace github.com/Luzifer/ots/pkg/customization => ../customization

require (
	github.com/Luzifer/go-openssl/v4 v4.2.5
	github.com/Luzifer/ots/pkg/customization v0.0.0-20260806231054-72d901945284
	github.com/sirupsen/logrus v1.9.4
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
