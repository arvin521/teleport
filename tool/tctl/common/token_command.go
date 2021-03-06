/*
Copyright 2015-2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"fmt"
	"time"

	"github.com/buger/goterm"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// TokenCommand implements `tctl token` group of commands
type TokenCommand struct {
	config *service.Config
	// token argument to 'tokens del' command
	token string
}

// onTokenList is called to execute "tokens del" command
func (c *TokenCommand) Del(client *auth.TunClient) error {
	if c.token == "" {
		return trace.Errorf("Need an argument: token")
	}
	if err := client.DeleteToken(c.token); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Token %s has been deleted\n", c.token)
	return nil
}

// onTokenList is called to execute "tokens ls" command
func (c *TokenCommand) List(client *auth.TunClient) error {
	tokens, err := client.GetTokens()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(tokens) == 0 {
		fmt.Println("No active tokens found.")
		return nil
	}
	tokensView := func() string {
		table := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(table, []string{"Token", "Role", "Expiry Time (UTC)"})
		for _, t := range tokens {
			expiry := "never"
			if t.Expires.Unix() > 0 {
				expiry = t.Expires.Format(time.RFC822)
			}
			fmt.Fprintf(table, "%v\t%v\t%s\n", t.Token, t.Roles.String(), expiry)
		}
		return table.String()
	}
	fmt.Printf(tokensView())
	return nil
}
