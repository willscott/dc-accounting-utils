package lib

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/mitchellh/go-homedir"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/urfave/cli/v2"
)

var ApiFlag = cli.StringFlag{
	Name:    "api",
	Usage:   "api endpoint, formatted as token:multiaddr",
	Value:   "",
	EnvVars: []string{"FULLNODE_API_INFO"},
}

func tryGetAPIFromHomeDir() ([]string, error) {
	p, err := homedir.Expand("~/.lotus")
	if err != nil {
		return nil, fmt.Errorf("could not find API from file system. specify explicitly - %w", err)
	}
	tokenPath := filepath.Join(p, "token")
	apiPath := filepath.Join(p, "api")

	token, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}
	apiData, err := os.ReadFile(apiPath)
	if err != nil {
		return nil, err
	}
	return []string{string(token), string(apiData)}, nil
}

func GetAPI(c *cli.Context) (api.FullNode, error) {
	var err error
	api := c.String(ApiFlag.Name)
	var addr, token string
	if strings.HasPrefix(api, "wss:") || strings.HasPrefix(api, "ws:") || strings.HasPrefix(api, "http:") || strings.HasPrefix(api, "https:") {
		addr = api
	} else {
		sp := strings.SplitN(api, ":", 2)
		if len(sp) != 2 {
			sp, err = tryGetAPIFromHomeDir()
			if err != nil {
				return nil, fmt.Errorf("invalid api value, missing token or address: %s", api)
			}
		}

		token = sp[0]
		ma, err := multiaddr.NewMultiaddr(sp[1])
		if err != nil {
			return nil, fmt.Errorf("could not parse provided multiaddr: %w", err)
		}

		_, dialAddr, err := manet.DialArgs(ma)
		if err != nil {
			return nil, fmt.Errorf("invalid api multiAddr: %w", err)
		}

		addr = "ws://" + dialAddr + "/rpc/v0"
	}
	headers := http.Header{}
	if len(token) != 0 {
		headers.Add("Authorization", "Bearer "+token)
	}

	node, _, err := client.NewFullNodeRPCV1(c.Context, addr, headers)
	if err != nil {
		return nil, fmt.Errorf("could not connect to api: %w", err)
	}
	return node, nil
}

func GetBlockstore(c *cli.Context) (api.FullNode, blockstore.Blockstore, error) {
	client, err := GetAPI(c)
	if err != nil {
		return nil, nil, err
	}
	return client, StoreFor(c.Context, client), nil
}
