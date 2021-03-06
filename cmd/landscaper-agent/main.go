// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"

	app "github.com/gardener/landscaper/cmd/landscaper-agent/app"
)

func main() {
	ctx := context.Background()
	defer ctx.Done()
	cmd := app.NewLandscaperAgentCommand(ctx)

	if err := cmd.Execute(); err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}
