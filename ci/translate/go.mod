module translate

go 1.25.7

toolchain go1.26.5

replace github.com/edsilegxrepo/ots/pkg/tplfunc => ../../pkg/tplfunc

require (
	github.com/edsilegxrepo/ots/pkg/tplfunc v0.0.0-20260806231054-72d901945284
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/sirupsen/logrus v1.9.4
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/kr/pretty v0.3.1 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)
