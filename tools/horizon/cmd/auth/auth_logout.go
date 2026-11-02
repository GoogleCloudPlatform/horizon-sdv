// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"fmt"
	"os"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

func NewAuthLogoutCmd() *cobra.Command {
	var authLogoutCmd = &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials",
		Long:  "Remove the cached token at ~/.config/horizon/token.json",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			if err := horizon.ClearTokenCache(); err != nil {
				return err
			}
			p, err2 := horizon.TokenCachePath()
			if err2 != nil {
				return err2
			}
			fmt.Fprintf(os.Stderr, "Removed %s (if present).\n", p)

			return nil
		},
	}

	return authLogoutCmd
}
