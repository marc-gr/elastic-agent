// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Code generated by beats/dev-tools/licenses/license_generate.go - DO NOT EDIT.

package licenses

import "fmt"

var Elastic = `
// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.
`

var Elasticv2 = `
// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.
`

func Find(name string) (string, error) {
	switch name {

	case "Elastic":
		return Elastic, nil
	case "Elasticv2":
		return Elasticv2, nil
	}
	return "", fmt.Errorf("unknown license: %s", name)
}