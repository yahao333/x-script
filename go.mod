module github.com/yahao333/x-script

go 1.23.2

require (
	github.com/lxn/walk v0.0.0-20210112085537-c389da54e794
	github.com/lxn/win v0.0.0-20210218163916-a377121e959e
	github.com/sirupsen/logrus v1.9.3
	golang.org/x/sys v0.27.0
)

require gopkg.in/Knetic/govaluate.v3 v3.0.0 // indirect

replace github.com/lxn/walk => github.com/yahao333/walk v0.2.4
