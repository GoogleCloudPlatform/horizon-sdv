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

package history

import (
	"encoding/json"
	"fmt"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

func NewHistoryCmd() *cobra.Command {
	var month string
	var historyCmd = &cobra.Command{
		Use:   "history [n]",
		Short: "Show recent CLI invocations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			n := horizon.HistoryDefaultLimit
			if len(args) == 1 {
				var err error
				n, err = horizon.ParseHistoryLimit(args[0])
				if err != nil {
					return err
				}
			}
			entries, err := horizon.ReadInvocationsForMonth(n, month)
			if err != nil {
				return err
			}
			if horizon.IsJSON() {
				b, err := json.MarshalIndent(entries, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}
			for _, e := range entries {
				fmt.Printf("%s  %s\n", e.Timestamp, e.Command)
			}
			return nil
		},
	}
	historyCmd.Flags().StringVar(&month, "month", "", fmt.Sprintf("Read YYYY-MM instead of the current UTC month (within the %d-month retention period)", horizon.HistoryRetentionMonths))

	return historyCmd
}
