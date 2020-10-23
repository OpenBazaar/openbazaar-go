package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/OpenBazaar/multiwallet/api"
	"github.com/OpenBazaar/multiwallet/api/pb"
	"github.com/jessevdk/go-flags"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func SetupCli(parser *flags.Parser) {
	// Add commands to parser
	parser.AddCommand("stop",
		"stop the wallet",
		"The stop command disconnects from peers and shuts down the wallet",
		&stop)
	parser.AddCommand("currentaddress",
		"get the current bitcoin address",
		"Returns the first unused address in the keychain\n\n"+
			"Args:\n"+
			"1. coinType (string)\n"+
			"2. purpose       (string default=external) The purpose for the address. Can be external for receiving from outside parties or internal for example, for change.\n\n"+
			"Examples:\n"+
			"> multiwallet currentaddress bitcoin\n"+
			"1DxGWC22a46VPEjq8YKoeVXSLzB7BA8sJS\n"+
			"> multiwallet currentaddress bitcoin internal\n"+
			"18zAxgfKx4NuTUGUEuB8p7FKgCYPM15DfS\n",
		&currentAddress)
	parser.AddCommand("newaddress",
		"get a new bitcoin address",
		"Returns a new unused address in the keychain. Use caution when using this function as generating too many new addresses may cause the keychain to extend further than the wallet's lookahead window, meaning it might fail to recover all transactions when restoring from seed. CurrentAddress is safer as it never extends past the lookahead window.\n\n"+
			"Args:\n"+
			"1. coinType (string)\n"+
			"2. purpose       (string default=external) The purpose for the address. Can be external for receiving from outside parties or internal for example, for change.\n\n"+
			"Examples:\n"+
			"> multiwallet newaddress bitcoin\n"+
			"1DxGWC22a46VPEjq8YKoeVXSLzB7BA8sJS\n"+
			"> multiwallet newaddress bitcoin internal\n"+
			"18zAxgfKx4NuTUGUEuB8p7FKgCYPM15DfS\n",
		&newAddress)
	parser.AddCommand("chaintip",
		"return the height of the chain",
		"Returns the height of the best chain of blocks",
		&chainTip)
	parser.AddCommand("dumptables",
		"print out the database tables",
		"Prints each row in the database tables",
		&dumpTables)
	parser.AddCommand("spend",
		"send bitcoins",
		"Send bitcoins to the given address\n\n"+
			"Args:\n"+
			"1. coinType      (string)\n"+
			"2. address       (string) The recipient's bitcoin address\n"+
			"3. amount        (integer) The amount to send in satoshi"+
			"4. feelevel      (string default=normal) The fee level: economic, normal, priority\n\n"+
			"5. memo          (string) The orderID\n"+
			"Examples:\n"+
			"> multiwallet spend bitcoin 1DxGWC22a46VPEjq8YKoeVXSLzB7BA8sJS 1000000\n"+
			"82bfd45f3564e0b5166ab9ca072200a237f78499576e9658b20b0ccd10ff325c 1a3w"+
			"> multiwallet spend bitcoin 1DxGWC22a46VPEjq8YKoeVXSLzB7BA8sJS 3000000000 priority\n"+
			"82bfd45f3564e0b5166ab9ca072200a237f78499576e9658b20b0ccd10ff325c 4wq2",
		&spend)
	parser.AddCommand("balance",
		"get the wallet's balances",
		"Returns the confirmed and unconfirmed balances for the specified coin",
		&balance)
}

func coinType(args []string) pb.CoinType {
	if len(args) == 0 {
		return pb.CoinType_BITCOIN
	}
	switch strings.ToLower(args[0]) {
	case "bitcoin":
		return pb.CoinType_BITCOIN
	case "bitcoincash":
		return pb.CoinType_BITCOIN_CASH
	case "zcash":
		return pb.CoinType_ZCASH
	case "litecoin":
		return pb.CoinType_LITECOIN
	case "ethereum":
		return pb.CoinType_ETHEREUM
	default:
		return pb.CoinType_BITCOIN
	}
}

func newGRPCClient() (pb.APIClient, *grpc.ClientConn, error) {
	// Set up a connection to the server.
	conn, err := grpc.Dial(api.Addr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	client := pb.NewAPIClient(conn)
	return client, conn, nil
}

type Stop struct{}

var stop Stop

func (x *Stop) Execute(args []string) error {
	client, conn, err := newGRPCClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	client.Stop(context.Background(), &pb.Empty{})
	return nil
}

type CurrentAddress struct{}

var currentAddress CurrentAddress

func (x *CurrentAddress) Execute(args []string) error {
	client, conn, err := newGRPCClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	var purpose pb.KeyPurpose
	userSelection := ""

	t := coinType(args)
	if len(args) == 1 {
		userSelection = args[0]
	} else if len(args) == 2 {
		userSelection = args[1]
	}
	switch strings.ToLower(userSelection) {
	case "internal":
		purpose = pb.KeyPurpose_INTERNAL
	case "external":
		purpose = pb.KeyPurpose_EXTERNAL
	default:
		purpose = pb.KeyPurpose_EXTERNAL
	}

	resp, err := client.CurrentAddress(context.Background(), &pb.KeySelection{Coin: t, Purpose: purpose})
	if err != nil {
		return err
	}
	fmt.Println(resp.Addr)
	return nil
}

type NewAddress struct{}

var newAddress NewAddress

func (x *NewAddress) Execute(args []string) error {
	client, conn, err := newGRPCClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	if len(args) == 0 {
		return errors.New("Must select coin type")
	}
	t := coinType(args)
	var purpose pb.KeyPurpose
	userSelection := ""
	if len(args) == 1 {
		userSelection = args[0]
	} else if len(args) == 2 {
		userSelection = args[1]
	}
	switch strings.ToLower(userSelection) {
	case "internal":
		purpose = pb.KeyPurpose_INTERNAL
	case "external":
		purpose = pb.KeyPurpose_EXTERNAL
	default:
		purpose = pb.KeyPurpose_EXTERNAL
	}
	resp, err := client.NewAddress(context.Background(), &pb.KeySelection{Coin: t, Purpose: purpose})
	if err != nil {
		return err
	}
	fmt.Println(resp.Addr)
	return nil
}

type ChainTip struct{}

var chainTip ChainTip

func (x *ChainTip) Execute(args []string) error {
	client, conn, err := newGRPCClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	if len(args) == 0 {
		return errors.New("Must select coin type")
	}
	t := coinType(args)
	resp, err := client.ChainTip(context.Background(), &pb.CoinSelection{Coin: t})
	if err != nil {
		return err
	}
	fmt.Println(resp.Height)
	return nil
}

type DumpTables struct{}

var dumpTables DumpTables

func (x *DumpTables) Execute(args []string) error {
	client, conn, err := newGRPCClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	if len(args) == 0 {
		return errors.New("Must select coin type")
	}
	t := coinType(args)
	resp, err := client.DumpTables(context.Background(), &pb.CoinSelection{Coin: t})
	if err != nil {
		return err
	}
	for {
		row, err := resp.Recv()
		if err != nil {
			// errors when no more rows and exits
			return err
		}
		fmt.Println(row.Data)
	}
}

type Spend struct{}

var spend Spend

func (x *Spend) Execute(args []string) error {
	var (
		address       string
		feeLevel      pb.FeeLevel
		referenceID   string
		userSelection string

		client, conn, err = newGRPCClient()
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	if len(args) == 0 {
		return errors.New("Must select coin type")
	}
	if len(args) > 4 {
		address = args[1]
		userSelection = args[3]
		referenceID = args[4]
	}
	if len(args) < 4 {
		return errors.New("Address and amount are required")
	}

	switch strings.ToLower(userSelection) {
	case "economic":
		feeLevel = pb.FeeLevel_ECONOMIC
	case "normal":
		feeLevel = pb.FeeLevel_NORMAL
	case "priority":
		feeLevel = pb.FeeLevel_PRIORITY
	default:
		feeLevel = pb.FeeLevel_NORMAL
	}

	amt, err := strconv.Atoi(args[2])
	if err != nil {
		return err
	}

	resp, err := client.Spend(context.Background(), &pb.SpendInfo{
		Coin:     coinType(args),
		Address:  address,
		Amount:   uint64(amt),
		FeeLevel: feeLevel,
		Memo:     referenceID,
	})
	if err != nil {
		return err
	}

	fmt.Println(resp.Hash)
	return nil
}

type Balance struct{}

var balance Balance

func (x *Balance) Execute(args []string) error {
	client, conn, err := newGRPCClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	if len(args) == 0 {
		return errors.New("Must select coin type")
	}
	t := coinType(args)
	resp, err := client.Balance(context.Background(), &pb.CoinSelection{Coin: t})
	if err != nil {
		return err
	}
	fmt.Printf("Confirmed: %d, Unconfirmed: %d\n", resp.Confirmed, resp.Unconfirmed)
	return nil
}
