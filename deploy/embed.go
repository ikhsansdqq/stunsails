// Package deploy holds the systemd unit template, embedded into the binary so
// `stunsails init` can install the service without a checkout.
package deploy

import _ "embed"

//go:embed stunsails.service.tmpl
var Unit string
