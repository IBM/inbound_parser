// print licenses, duh //
package main

import (
	"fmt"
)

func printLicenses() {
	fmt.Print(`
*********************************************
* 3rd-party software used by inbound_parser *
*********************************************

github.com/andygrunwald/go-jira,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/andygrunwald/go-jira/LICENSE,MIT
github.com/cention-sany/utf7,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/cention-sany/utf7/LICENSE,BSD-3-Clause
github.com/fatih/structs,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/fatih/structs/LICENSE,MIT
github.com/gogs/chardet,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/gogs/chardet/LICENSE,MIT
github.com/golang-jwt/jwt/v4,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/golang-jwt/jwt/v4/LICENSE,MIT
github.com/google/go-querystring/query,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/google/go-querystring/LICENSE,BSD-3-Clause
github.com/h2non/filetype,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/h2non/filetype/LICENSE,MIT
github.com/jaytaylor/html2text,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/jaytaylor/html2text/LICENSE,MIT
github.com/jhillyerd/enmime/v2,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/jhillyerd/enmime/v2/LICENSE,MIT
github.com/mattn/go-runewidth,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/mattn/go-runewidth/LICENSE,MIT
github.com/mattn/go-sqlite3,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/mattn/go-sqlite3/LICENSE,MIT
github.com/olekukonko/tablewriter,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/olekukonko/tablewriter/LICENSE.md,MIT
github.com/pkg/errors,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/pkg/errors/LICENSE,BSD-2-Clause
github.com/rivo/uniseg,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/rivo/uniseg/LICENSE.txt,MIT
github.com/ssor/bom,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/ssor/bom/LICENSE,MIT
github.com/trivago/tgo,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/github.com/trivago/tgo/LICENSE,Apache-2.0
github.ibmgcloud.net/dth/inbound_parser,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/licenses.go,Apache-2.0
golang.org/x/net/html,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/golang.org/x/net/LICENSE,BSD-3-Clause
golang.org/x/text,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/golang.org/x/text/LICENSE,BSD-3-Clause
gopkg.in/gomail.v2,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/gopkg.in/gomail.v2/LICENSE,MIT
gopkg.in/yaml.v2,https://github.ibmgcloud.net/dth/inbound_parser/blob/HEAD/vendor/gopkg.in/yaml.v2/LICENSE,Apache-2.0
`)
}
