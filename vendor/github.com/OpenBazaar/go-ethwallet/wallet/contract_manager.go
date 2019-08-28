// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package wallet

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = abi.U256
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// RegistryABI is the input ABI used to generate the binding from.
const RegistryABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"contractName\",\"type\":\"string\"},{\"name\":\"versionName\",\"type\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\"},{\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"addVersion\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"contractName\",\"type\":\"string\"},{\"name\":\"versionName\",\"type\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\"},{\"name\":\"bugLevel\",\"type\":\"uint8\"}],\"name\":\"updateVersion\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getTotalContractCount\",\"outputs\":[{\"name\":\"count\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"contractName\",\"type\":\"string\"}],\"name\":\"getRecommendedVersion\",\"outputs\":[{\"name\":\"versionName\",\"type\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\"},{\"name\":\"bugLevel\",\"type\":\"uint8\"},{\"name\":\"implementation\",\"type\":\"address\"},{\"name\":\"dateAdded\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"contractName\",\"type\":\"string\"},{\"name\":\"versionName\",\"type\":\"string\"}],\"name\":\"getVersionDetails\",\"outputs\":[{\"name\":\"versionString\",\"type\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\"},{\"name\":\"bugLevel\",\"type\":\"uint8\"},{\"name\":\"implementation\",\"type\":\"address\"},{\"name\":\"dateAdded\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"getContractAtIndex\",\"outputs\":[{\"name\":\"contractName\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"contractName\",\"type\":\"string\"}],\"name\":\"removeRecommendedVersion\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"contractName\",\"type\":\"string\"},{\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"getVersionAtIndex\",\"outputs\":[{\"name\":\"versionName\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"contractName\",\"type\":\"string\"},{\"name\":\"versionName\",\"type\":\"string\"}],\"name\":\"markRecommendedVersion\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"contractName\",\"type\":\"string\"}],\"name\":\"getVersionCountForContract\",\"outputs\":[{\"name\":\"count\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"contractName\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"versionName\",\"type\":\"string\"},{\"indexed\":true,\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"VersionAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"contractName\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"versionName\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"status\",\"type\":\"uint8\"},{\"indexed\":false,\"name\":\"bugLevel\",\"type\":\"uint8\"}],\"name\":\"VersionUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"contractName\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"versionName\",\"type\":\"string\"}],\"name\":\"VersionRecommended\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"contractName\",\"type\":\"string\"}],\"name\":\"RecommendedVersionRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"previousOwner\",\"type\":\"address\"}],\"name\":\"OwnershipRenounced\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"}]=======openzeppelin-solidity/contracts/ownership/Ownable.sol:Ownable=======[{\"constant\":false,\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"previousOwner\",\"type\":\"address\"}],\"name\":\"OwnershipRenounced\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"}]=======openzeppelin-solidity/contracts/utils/Address.sol:Address=======[]"

// RegistryBin is the compiled bytecode used for deploying new contracts.
const RegistryBin = `6080604052336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550612460806100536000396000f3006080604052600436106100c5576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680633de5311a146100ca5780635c047fa21461014a578063715018a6146101b7578063751f592b146101ce5780637e27634e146101f95780638ad030c1146103115780638da5cb5b146104415780639b534f1814610498578063acd820a81461053e578063af2c7fa314610579578063ca2e7cf314610637578063f2fde38b1461068a578063ff93dab4146106cd575b600080fd5b3480156100d657600080fd5b50610148600480360381019080803590602001908201803590602001919091929391929390803590602001908201803590602001919091929391929390803560ff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061071c565b005b34801561015657600080fd5b506101b5600480360381019080803590602001908201803590602001919091929391929390803590602001908201803590602001919091929391929390803560ff169060200190929190803560ff169060200190929190505050610e7b565b005b3480156101c357600080fd5b506101cc6113a0565b005b3480156101da57600080fd5b506101e36114a2565b6040518082815260200191505060405180910390f35b34801561020557600080fd5b506102326004803603810190808035906020019082018035906020019190919293919293905050506114b2565b604051808060200186600381111561024657fe5b60ff16815260200185600481111561025a57fe5b60ff1681526020018473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001838152602001828103825287818151815260200191508051906020019080838360005b838110156102d25780820151818401526020810190506102b7565b50505050905090810190601f1680156102ff5780820380516001836020036101000a031916815260200191505b50965050505050505060405180910390f35b34801561031d57600080fd5b5061036260048036038101908080359060200190820180359060200191909192939192939080359060200190820180359060200191909192939192939050505061179d565b604051808060200186600381111561037657fe5b60ff16815260200185600481111561038a57fe5b60ff1681526020018473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001838152602001828103825287818151815260200191508051906020019080838360005b838110156104025780820151818401526020810190506103e7565b50505050905090810190601f16801561042f5780820380516001836020036101000a031916815260200191505b50965050505050505060405180910390f35b34801561044d57600080fd5b50610456611901565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156104a457600080fd5b506104c360048036038101908080359060200190929190505050611926565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156105035780820151818401526020810190506104e8565b50505050905090810190601f1680156105305780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561054a57600080fd5b506105776004803603810190808035906020019082018035906020019190919293919293905050506119e4565b005b34801561058557600080fd5b506105bc60048036038101908080359060200190820180359060200191909192939192939080359060200190929190505050611be6565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156105fc5780820151818401526020810190506105e1565b50505050905090810190601f1680156106295780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561064357600080fd5b50610688600480360381019080803590602001908201803590602001919091929391929390803590602001908201803590602001919091929391929390505050611cc9565b005b34801561069657600080fd5b506106cb600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061211d565b005b3480156106d957600080fd5b50610706600480360381019080803590602001908201803590602001919091929391929390505050612184565b6040518082815260200191505060405180910390f35b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561077757600080fd5b80600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610843576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260258152602001807f5468652070726f7669646564206164647265737320697320746865203020616481526020017f647265737300000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b6000858590501115156108be576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601e8152602001807f456d70747920737472696e67207061737365642061732076657273696f6e000081525060200191505060405180910390fd5b60008787905011151561095f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260248152602001807f456d70747920737472696e672070617373656420617320636f6e74726163742081526020017f6e616d650000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b610968826121ba565b1515610a02576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260368152602001807f43616e6e6f742073657420616e20696d706c656d656e746174696f6e20746f2081526020017f61206e6f6e2d636f6e747261637420616464726573730000000000000000000081525060400191505060405180910390fd5b6002878760405180838380828437820191505092505050908152602001604051809103902060009054906101000a900460ff161515610abc576001878790918060018154018082558091505090600182039060005260206000200160009091929390919293909192909192509190610a7b9291906122c7565b505060016002888860405180838380828437820191505092505050908152602001604051809103902060006101000a81548160ff0219169083151502179055505b600073ffffffffffffffffffffffffffffffffffffffff1660048888604051808383808284378201915050925050509081526020016040518091039020868660405180838380828437820191505092505050908152602001604051809103902060010160029054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16141515610bf1576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260238152602001807f56657273696f6e20616c72656164792065786973747320666f7220636f6e747281526020017f616374000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b60038787604051808383808284378201915050925050509081526020016040518091039020858590918060018154018082558091505090600182039060005260206000200160009091929390919293909192909192509190610c549291906122c7565b505060a06040519081016040528086868080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050508152602001846003811115610ca657fe5b815260200160006004811115610cb857fe5b81526020018373ffffffffffffffffffffffffffffffffffffffff168152602001428152506004888860405180838380828437820191505092505050908152602001604051809103902086866040518083838082843782019150509250505090815260200160405180910390206000820151816000019080519060200190610d41929190612347565b5060208201518160010160006101000a81548160ff02191690836003811115610d6657fe5b021790555060408201518160010160016101000a81548160ff02191690836004811115610d8f57fe5b021790555060608201518160010160026101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550608082015181600201559050508173ffffffffffffffffffffffffffffffffffffffff167f337b109e3f497728f2bdd27545c9ed1cb52ed4a4103cc94da88b868879c982e2888888886040518080602001806020018381038352878782818152602001925080828437820191505083810382528585828181526020019250808284378201915050965050505050505060405180910390a250505050505050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515610ed657600080fd5b85858080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050506002816040518082805190602001908083835b602083101515610f415780518252602082019150602081019050602083039250610f1c565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060009054906101000a900460ff161515610ff6576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f436f6e747261637420646f6573206e6f7420657869737473000000000000000081525060200191505060405180910390fd5b86868080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505085858080601f016020809104026020016040519081016040528093929190818152602001838380828437820191505050505050600073ffffffffffffffffffffffffffffffffffffffff166004836040518082805190602001908083835b6020831015156110ac5780518252602082019150602081019050602083039250611087565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020826040518082805190602001908083835b60208310151561111557805182526020820191506020810190506020830392506110f0565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060010160029054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415151561121e576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260248152602001807f56657273696f6e20646f6573206e6f742065786973747320666f7220636f6e7481526020017f726163740000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b8460048a8a604051808383808284378201915050925050509081526020016040518091039020888860405180838380828437820191505092505050908152602001604051809103902060010160006101000a81548160ff0219169083600381111561128557fe5b02179055508360048a8a604051808383808284378201915050925050509081526020016040518091039020888860405180838380828437820191505092505050908152602001604051809103902060010160016101000a81548160ff021916908360048111156112f157fe5b02179055507f0acf3e1a00b57bfc05ebf65957f42293847dc0938bfa1744660d6df56036d75189898989898960405180806020018060200185600381111561133557fe5b60ff16815260200184600481111561134957fe5b60ff16815260200183810383528989828181526020019250808284378201915050838103825287878281815260200192508082843782019150509850505050505050505060405180910390a1505050505050505050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156113fb57600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167ff8df31144d9c2f0f6b59d69b8b98abd5459d07f2742c4df920b25aae33c6482060405160405180910390a260008060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550565b6000600180549050905080905090565b6060600080600080600087878080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050506002816040518082805190602001908083835b6020831015156115275780518252602082019150602081019050602083039250611502565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060009054906101000a900460ff1615156115dc576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f436f6e747261637420646f6573206e6f7420657869737473000000000000000081525060200191505060405180910390fd5b600589896040518083838082843782019150509250505090815260200160405180910390208054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156116955780601f1061166a57610100808354040283529160200191611695565b820191906000526020600020905b81548152906001019060200180831161167857829003601f168201915b5050505050965060048989604051808383808284378201915050925050509081526020016040518091039020876040518082805190602001908083835b6020831015156116f757805182526020820191506020810190506020830392506116d2565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902091508160010160009054906101000a900460ff1695508160010160019054906101000a900460ff1694508160010160029054906101000a900473ffffffffffffffffffffffffffffffffffffffff1693508160020154925086868686869650965096509650965050509295509295909350565b6060600080600080600060048a8a60405180838380828437820191505092505050908152602001604051809103902088886040518083838082843782019150509250505090815260200160405180910390209050806000018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156118895780601f1061185e57610100808354040283529160200191611889565b820191906000526020600020905b81548152906001019060200180831161186c57829003601f168201915b505050505095508060010160009054906101000a900460ff1694508060010160019054906101000a900460ff1693508060010160029054906101000a900473ffffffffffffffffffffffffffffffffffffffff1692508060020154915085858585859550955095509550955050945094509450945094565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b606060018281548110151561193757fe5b906000526020600020018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156119d55780601f106119aa576101008083540402835291602001916119d5565b820191906000526020600020905b8154815290600101906020018083116119b857829003601f168201915b50505050509050809050919050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515611a3f57600080fd5b81818080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050506002816040518082805190602001908083835b602083101515611aaa5780518252602082019150602081019050602083039250611a85565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060009054906101000a900460ff161515611b5f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f436f6e747261637420646f6573206e6f7420657869737473000000000000000081525060200191505060405180910390fd5b600583836040518083838082843782019150509250505090815260200160405180910390206000611b9091906123c7565b7f07b20feb74e0118ee3c73d4cb8d0eb4da169604c68aa233293b094cedcd225f28383604051808060200182810382528484828181526020019250808284378201915050935050505060405180910390a1505050565b60606003848460405180838380828437820191505092505050908152602001604051809103902082815481101515611c1a57fe5b906000526020600020018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015611cb85780601f10611c8d57610100808354040283529160200191611cb8565b820191906000526020600020905b815481529060010190602001808311611c9b57829003601f168201915b505050505090508090509392505050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515611d2457600080fd5b83838080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050506002816040518082805190602001908083835b602083101515611d8f5780518252602082019150602081019050602083039250611d6a565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060009054906101000a900460ff161515611e44576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f436f6e747261637420646f6573206e6f7420657869737473000000000000000081525060200191505060405180910390fd5b84848080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505083838080601f016020809104026020016040519081016040528093929190818152602001838380828437820191505050505050600073ffffffffffffffffffffffffffffffffffffffff166004836040518082805190602001908083835b602083101515611efa5780518252602082019150602081019050602083039250611ed5565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020826040518082805190602001908083835b602083101515611f635780518252602082019150602081019050602083039250611f3e565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060010160029054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415151561206c576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260248152602001807f56657273696f6e20646f6573206e6f742065786973747320666f7220636f6e7481526020017f726163740000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b84846005898960405180838380828437820191505092505050908152602001604051809103902091906120a09291906122c7565b507fb318550bf93edf51de4bae84db3deabd2a866cc407435a72317ca2503e2a07a6878787876040518080602001806020018381038352878782818152602001925080828437820191505083810382528585828181526020019250808284378201915050965050505050505060405180910390a150505050505050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561217857600080fd5b612181816121cd565b50565b60006003838360405180838380828437820191505092505050908152602001604051809103902080549050905080905092915050565b600080823b905060008111915050919050565b600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161415151561220957600080fd5b8073ffffffffffffffffffffffffffffffffffffffff166000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a3806000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061230857803560ff1916838001178555612336565b82800160010185558215612336579182015b8281111561233557823582559160200191906001019061231a565b5b509050612343919061240f565b5090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061238857805160ff19168380011785556123b6565b828001600101855582156123b6579182015b828111156123b557825182559160200191906001019061239a565b5b5090506123c3919061240f565b5090565b50805460018160011615610100020316600290046000825580601f106123ed575061240c565b601f01602090049060005260206000209081019061240b919061240f565b5b50565b61243191905b8082111561242d576000816000905550600101612415565b5090565b905600a165627a7a723058202e2b25c5a471a67687142064b12a7bce9573e8ae12b645365a9c6215707bc07d0029

======= openzeppelin-solidity/contracts/ownership/Ownable.sol:Ownable =======
608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506103c1806100606000396000f300608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063715018a61461005c5780638da5cb5b14610073578063f2fde38b146100ca575b600080fd5b34801561006857600080fd5b5061007161010d565b005b34801561007f57600080fd5b5061008861020f565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156100d657600080fd5b5061010b600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610234565b005b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561016857600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167ff8df31144d9c2f0f6b59d69b8b98abd5459d07f2742c4df920b25aae33c6482060405160405180910390a260008060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561028f57600080fd5b6102988161029b565b50565b600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff16141515156102d757600080fd5b8073ffffffffffffffffffffffffffffffffffffffff166000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a3806000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550505600a165627a7a7230582077e73d4b401459c2c062401742582a44912b8b235987c864fea6d190a5b48b600029

======= openzeppelin-solidity/contracts/utils/Address.sol:Address =======
604c602c600b82828239805160001a60731460008114601c57601e565bfe5b5030600052607381538281f30073000000000000000000000000000000000000000030146080604052600080fd00a165627a7a7230582083c43d1ee71d56e3cc73e85663e5bb2db836c108d53796dbcefd5ffb60903be10029`

// DeployRegistry deploys a new Ethereum contract, binding an instance of Registry to it.
func DeployRegistry(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Registry, error) {
	parsed, err := abi.JSON(strings.NewReader(RegistryABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(RegistryBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Registry{RegistryCaller: RegistryCaller{contract: contract}, RegistryTransactor: RegistryTransactor{contract: contract}, RegistryFilterer: RegistryFilterer{contract: contract}}, nil
}

// Registry is an auto generated Go binding around an Ethereum contract.
type Registry struct {
	RegistryCaller     // Read-only binding to the contract
	RegistryTransactor // Write-only binding to the contract
	RegistryFilterer   // Log filterer for contract events
}

// RegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type RegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RegistrySession struct {
	Contract     *Registry         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RegistryCallerSession struct {
	Contract *RegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// RegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RegistryTransactorSession struct {
	Contract     *RegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// RegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type RegistryRaw struct {
	Contract *Registry // Generic contract binding to access the raw methods on
}

// RegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RegistryCallerRaw struct {
	Contract *RegistryCaller // Generic read-only contract binding to access the raw methods on
}

// RegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RegistryTransactorRaw struct {
	Contract *RegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRegistry creates a new instance of Registry, bound to a specific deployed contract.
func NewRegistry(address common.Address, backend bind.ContractBackend) (*Registry, error) {
	contract, err := bindRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Registry{RegistryCaller: RegistryCaller{contract: contract}, RegistryTransactor: RegistryTransactor{contract: contract}, RegistryFilterer: RegistryFilterer{contract: contract}}, nil
}

// NewRegistryCaller creates a new read-only instance of Registry, bound to a specific deployed contract.
func NewRegistryCaller(address common.Address, caller bind.ContractCaller) (*RegistryCaller, error) {
	contract, err := bindRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RegistryCaller{contract: contract}, nil
}

// NewRegistryTransactor creates a new write-only instance of Registry, bound to a specific deployed contract.
func NewRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*RegistryTransactor, error) {
	contract, err := bindRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RegistryTransactor{contract: contract}, nil
}

// NewRegistryFilterer creates a new log filterer instance of Registry, bound to a specific deployed contract.
func NewRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*RegistryFilterer, error) {
	contract, err := bindRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RegistryFilterer{contract: contract}, nil
}

// bindRegistry binds a generic wrapper to an already deployed contract.
func bindRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(RegistryABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Registry *RegistryRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Registry.Contract.RegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Registry *RegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Registry.Contract.RegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Registry *RegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Registry.Contract.RegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Registry *RegistryCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Registry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Registry *RegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Registry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Registry *RegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Registry.Contract.contract.Transact(opts, method, params...)
}

// GetContractAtIndex is a free data retrieval call binding the contract method 0x9b534f18.
//
// Solidity: function getContractAtIndex(uint256 index) constant returns(string contractName)
func (_Registry *RegistryCaller) GetContractAtIndex(opts *bind.CallOpts, index *big.Int) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _Registry.contract.Call(opts, out, "getContractAtIndex", index)
	return *ret0, err
}

// GetContractAtIndex is a free data retrieval call binding the contract method 0x9b534f18.
//
// Solidity: function getContractAtIndex(uint256 index) constant returns(string contractName)
func (_Registry *RegistrySession) GetContractAtIndex(index *big.Int) (string, error) {
	return _Registry.Contract.GetContractAtIndex(&_Registry.CallOpts, index)
}

// GetContractAtIndex is a free data retrieval call binding the contract method 0x9b534f18.
//
// Solidity: function getContractAtIndex(uint256 index) constant returns(string contractName)
func (_Registry *RegistryCallerSession) GetContractAtIndex(index *big.Int) (string, error) {
	return _Registry.Contract.GetContractAtIndex(&_Registry.CallOpts, index)
}

// GetRecommendedVersion is a free data retrieval call binding the contract method 0x7e27634e.
//
// Solidity: function getRecommendedVersion(string contractName) constant returns(string versionName, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistryCaller) GetRecommendedVersion(opts *bind.CallOpts, contractName string) (struct {
	VersionName    string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	ret := new(struct {
		VersionName    string
		Status         uint8
		BugLevel       uint8
		Implementation common.Address
		DateAdded      *big.Int
	})
	out := ret
	err := _Registry.contract.Call(opts, out, "getRecommendedVersion", contractName)
	return *ret, err
}

// GetRecommendedVersion is a free data retrieval call binding the contract method 0x7e27634e.
//
// Solidity: function getRecommendedVersion(string contractName) constant returns(string versionName, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistrySession) GetRecommendedVersion(contractName string) (struct {
	VersionName    string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	return _Registry.Contract.GetRecommendedVersion(&_Registry.CallOpts, contractName)
}

// GetRecommendedVersion is a free data retrieval call binding the contract method 0x7e27634e.
//
// Solidity: function getRecommendedVersion(string contractName) constant returns(string versionName, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistryCallerSession) GetRecommendedVersion(contractName string) (struct {
	VersionName    string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	return _Registry.Contract.GetRecommendedVersion(&_Registry.CallOpts, contractName)
}

// GetTotalContractCount is a free data retrieval call binding the contract method 0x751f592b.
//
// Solidity: function getTotalContractCount() constant returns(uint256 count)
func (_Registry *RegistryCaller) GetTotalContractCount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Registry.contract.Call(opts, out, "getTotalContractCount")
	return *ret0, err
}

// GetTotalContractCount is a free data retrieval call binding the contract method 0x751f592b.
//
// Solidity: function getTotalContractCount() constant returns(uint256 count)
func (_Registry *RegistrySession) GetTotalContractCount() (*big.Int, error) {
	return _Registry.Contract.GetTotalContractCount(&_Registry.CallOpts)
}

// GetTotalContractCount is a free data retrieval call binding the contract method 0x751f592b.
//
// Solidity: function getTotalContractCount() constant returns(uint256 count)
func (_Registry *RegistryCallerSession) GetTotalContractCount() (*big.Int, error) {
	return _Registry.Contract.GetTotalContractCount(&_Registry.CallOpts)
}

// GetVersionAtIndex is a free data retrieval call binding the contract method 0xaf2c7fa3.
//
// Solidity: function getVersionAtIndex(string contractName, uint256 index) constant returns(string versionName)
func (_Registry *RegistryCaller) GetVersionAtIndex(opts *bind.CallOpts, contractName string, index *big.Int) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _Registry.contract.Call(opts, out, "getVersionAtIndex", contractName, index)
	return *ret0, err
}

// GetVersionAtIndex is a free data retrieval call binding the contract method 0xaf2c7fa3.
//
// Solidity: function getVersionAtIndex(string contractName, uint256 index) constant returns(string versionName)
func (_Registry *RegistrySession) GetVersionAtIndex(contractName string, index *big.Int) (string, error) {
	return _Registry.Contract.GetVersionAtIndex(&_Registry.CallOpts, contractName, index)
}

// GetVersionAtIndex is a free data retrieval call binding the contract method 0xaf2c7fa3.
//
// Solidity: function getVersionAtIndex(string contractName, uint256 index) constant returns(string versionName)
func (_Registry *RegistryCallerSession) GetVersionAtIndex(contractName string, index *big.Int) (string, error) {
	return _Registry.Contract.GetVersionAtIndex(&_Registry.CallOpts, contractName, index)
}

// GetVersionCountForContract is a free data retrieval call binding the contract method 0xff93dab4.
//
// Solidity: function getVersionCountForContract(string contractName) constant returns(uint256 count)
func (_Registry *RegistryCaller) GetVersionCountForContract(opts *bind.CallOpts, contractName string) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Registry.contract.Call(opts, out, "getVersionCountForContract", contractName)
	return *ret0, err
}

// GetVersionCountForContract is a free data retrieval call binding the contract method 0xff93dab4.
//
// Solidity: function getVersionCountForContract(string contractName) constant returns(uint256 count)
func (_Registry *RegistrySession) GetVersionCountForContract(contractName string) (*big.Int, error) {
	return _Registry.Contract.GetVersionCountForContract(&_Registry.CallOpts, contractName)
}

// GetVersionCountForContract is a free data retrieval call binding the contract method 0xff93dab4.
//
// Solidity: function getVersionCountForContract(string contractName) constant returns(uint256 count)
func (_Registry *RegistryCallerSession) GetVersionCountForContract(contractName string) (*big.Int, error) {
	return _Registry.Contract.GetVersionCountForContract(&_Registry.CallOpts, contractName)
}

// GetVersionDetails is a free data retrieval call binding the contract method 0x8ad030c1.
//
// Solidity: function getVersionDetails(string contractName, string versionName) constant returns(string versionString, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistryCaller) GetVersionDetails(opts *bind.CallOpts, contractName string, versionName string) (struct {
	VersionString  string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	ret := new(struct {
		VersionString  string
		Status         uint8
		BugLevel       uint8
		Implementation common.Address
		DateAdded      *big.Int
	})
	out := ret
	err := _Registry.contract.Call(opts, out, "getVersionDetails", contractName, versionName)
	return *ret, err
}

// GetVersionDetails is a free data retrieval call binding the contract method 0x8ad030c1.
//
// Solidity: function getVersionDetails(string contractName, string versionName) constant returns(string versionString, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistrySession) GetVersionDetails(contractName string, versionName string) (struct {
	VersionString  string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	return _Registry.Contract.GetVersionDetails(&_Registry.CallOpts, contractName, versionName)
}

// GetVersionDetails is a free data retrieval call binding the contract method 0x8ad030c1.
//
// Solidity: function getVersionDetails(string contractName, string versionName) constant returns(string versionString, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistryCallerSession) GetVersionDetails(contractName string, versionName string) (struct {
	VersionString  string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	return _Registry.Contract.GetVersionDetails(&_Registry.CallOpts, contractName, versionName)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Registry *RegistryCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Registry.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Registry *RegistrySession) Owner() (common.Address, error) {
	return _Registry.Contract.Owner(&_Registry.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Registry *RegistryCallerSession) Owner() (common.Address, error) {
	return _Registry.Contract.Owner(&_Registry.CallOpts)
}

// AddVersion is a paid mutator transaction binding the contract method 0x3de5311a.
//
// Solidity: function addVersion(string contractName, string versionName, uint8 status, address implementation) returns()
func (_Registry *RegistryTransactor) AddVersion(opts *bind.TransactOpts, contractName string, versionName string, status uint8, implementation common.Address) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "addVersion", contractName, versionName, status, implementation)
}

// AddVersion is a paid mutator transaction binding the contract method 0x3de5311a.
//
// Solidity: function addVersion(string contractName, string versionName, uint8 status, address implementation) returns()
func (_Registry *RegistrySession) AddVersion(contractName string, versionName string, status uint8, implementation common.Address) (*types.Transaction, error) {
	return _Registry.Contract.AddVersion(&_Registry.TransactOpts, contractName, versionName, status, implementation)
}

// AddVersion is a paid mutator transaction binding the contract method 0x3de5311a.
//
// Solidity: function addVersion(string contractName, string versionName, uint8 status, address implementation) returns()
func (_Registry *RegistryTransactorSession) AddVersion(contractName string, versionName string, status uint8, implementation common.Address) (*types.Transaction, error) {
	return _Registry.Contract.AddVersion(&_Registry.TransactOpts, contractName, versionName, status, implementation)
}

// MarkRecommendedVersion is a paid mutator transaction binding the contract method 0xca2e7cf3.
//
// Solidity: function markRecommendedVersion(string contractName, string versionName) returns()
func (_Registry *RegistryTransactor) MarkRecommendedVersion(opts *bind.TransactOpts, contractName string, versionName string) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "markRecommendedVersion", contractName, versionName)
}

// MarkRecommendedVersion is a paid mutator transaction binding the contract method 0xca2e7cf3.
//
// Solidity: function markRecommendedVersion(string contractName, string versionName) returns()
func (_Registry *RegistrySession) MarkRecommendedVersion(contractName string, versionName string) (*types.Transaction, error) {
	return _Registry.Contract.MarkRecommendedVersion(&_Registry.TransactOpts, contractName, versionName)
}

// MarkRecommendedVersion is a paid mutator transaction binding the contract method 0xca2e7cf3.
//
// Solidity: function markRecommendedVersion(string contractName, string versionName) returns()
func (_Registry *RegistryTransactorSession) MarkRecommendedVersion(contractName string, versionName string) (*types.Transaction, error) {
	return _Registry.Contract.MarkRecommendedVersion(&_Registry.TransactOpts, contractName, versionName)
}

// RemoveRecommendedVersion is a paid mutator transaction binding the contract method 0xacd820a8.
//
// Solidity: function removeRecommendedVersion(string contractName) returns()
func (_Registry *RegistryTransactor) RemoveRecommendedVersion(opts *bind.TransactOpts, contractName string) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "removeRecommendedVersion", contractName)
}

// RemoveRecommendedVersion is a paid mutator transaction binding the contract method 0xacd820a8.
//
// Solidity: function removeRecommendedVersion(string contractName) returns()
func (_Registry *RegistrySession) RemoveRecommendedVersion(contractName string) (*types.Transaction, error) {
	return _Registry.Contract.RemoveRecommendedVersion(&_Registry.TransactOpts, contractName)
}

// RemoveRecommendedVersion is a paid mutator transaction binding the contract method 0xacd820a8.
//
// Solidity: function removeRecommendedVersion(string contractName) returns()
func (_Registry *RegistryTransactorSession) RemoveRecommendedVersion(contractName string) (*types.Transaction, error) {
	return _Registry.Contract.RemoveRecommendedVersion(&_Registry.TransactOpts, contractName)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Registry *RegistryTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Registry *RegistrySession) RenounceOwnership() (*types.Transaction, error) {
	return _Registry.Contract.RenounceOwnership(&_Registry.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Registry *RegistryTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _Registry.Contract.RenounceOwnership(&_Registry.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Registry *RegistryTransactor) TransferOwnership(opts *bind.TransactOpts, _newOwner common.Address) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "transferOwnership", _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Registry *RegistrySession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _Registry.Contract.TransferOwnership(&_Registry.TransactOpts, _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address _newOwner) returns()
func (_Registry *RegistryTransactorSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _Registry.Contract.TransferOwnership(&_Registry.TransactOpts, _newOwner)
}

// UpdateVersion is a paid mutator transaction binding the contract method 0x5c047fa2.
//
// Solidity: function updateVersion(string contractName, string versionName, uint8 status, uint8 bugLevel) returns()
func (_Registry *RegistryTransactor) UpdateVersion(opts *bind.TransactOpts, contractName string, versionName string, status uint8, bugLevel uint8) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "updateVersion", contractName, versionName, status, bugLevel)
}

// UpdateVersion is a paid mutator transaction binding the contract method 0x5c047fa2.
//
// Solidity: function updateVersion(string contractName, string versionName, uint8 status, uint8 bugLevel) returns()
func (_Registry *RegistrySession) UpdateVersion(contractName string, versionName string, status uint8, bugLevel uint8) (*types.Transaction, error) {
	return _Registry.Contract.UpdateVersion(&_Registry.TransactOpts, contractName, versionName, status, bugLevel)
}

// UpdateVersion is a paid mutator transaction binding the contract method 0x5c047fa2.
//
// Solidity: function updateVersion(string contractName, string versionName, uint8 status, uint8 bugLevel) returns()
func (_Registry *RegistryTransactorSession) UpdateVersion(contractName string, versionName string, status uint8, bugLevel uint8) (*types.Transaction, error) {
	return _Registry.Contract.UpdateVersion(&_Registry.TransactOpts, contractName, versionName, status, bugLevel)
}

// RegistryOwnershipRenouncedIterator is returned from FilterOwnershipRenounced and is used to iterate over the raw logs and unpacked data for OwnershipRenounced events raised by the Registry contract.
type RegistryOwnershipRenouncedIterator struct {
	Event *RegistryOwnershipRenounced // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RegistryOwnershipRenouncedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryOwnershipRenounced)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RegistryOwnershipRenounced)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RegistryOwnershipRenouncedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryOwnershipRenouncedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryOwnershipRenounced represents a OwnershipRenounced event raised by the Registry contract.
type RegistryOwnershipRenounced struct {
	PreviousOwner common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipRenounced is a free log retrieval operation binding the contract event 0xf8df31144d9c2f0f6b59d69b8b98abd5459d07f2742c4df920b25aae33c64820.
//
// Solidity: event OwnershipRenounced(address indexed previousOwner)
func (_Registry *RegistryFilterer) FilterOwnershipRenounced(opts *bind.FilterOpts, previousOwner []common.Address) (*RegistryOwnershipRenouncedIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}

	logs, sub, err := _Registry.contract.FilterLogs(opts, "OwnershipRenounced", previousOwnerRule)
	if err != nil {
		return nil, err
	}
	return &RegistryOwnershipRenouncedIterator{contract: _Registry.contract, event: "OwnershipRenounced", logs: logs, sub: sub}, nil
}

// WatchOwnershipRenounced is a free log subscription operation binding the contract event 0xf8df31144d9c2f0f6b59d69b8b98abd5459d07f2742c4df920b25aae33c64820.
//
// Solidity: event OwnershipRenounced(address indexed previousOwner)
func (_Registry *RegistryFilterer) WatchOwnershipRenounced(opts *bind.WatchOpts, sink chan<- *RegistryOwnershipRenounced, previousOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}

	logs, sub, err := _Registry.contract.WatchLogs(opts, "OwnershipRenounced", previousOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryOwnershipRenounced)
				if err := _Registry.contract.UnpackLog(event, "OwnershipRenounced", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RegistryOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Registry contract.
type RegistryOwnershipTransferredIterator struct {
	Event *RegistryOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RegistryOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RegistryOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RegistryOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryOwnershipTransferred represents a OwnershipTransferred event raised by the Registry contract.
type RegistryOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Registry *RegistryFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*RegistryOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Registry.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &RegistryOwnershipTransferredIterator{contract: _Registry.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Registry *RegistryFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *RegistryOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Registry.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryOwnershipTransferred)
				if err := _Registry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RegistryRecommendedVersionRemovedIterator is returned from FilterRecommendedVersionRemoved and is used to iterate over the raw logs and unpacked data for RecommendedVersionRemoved events raised by the Registry contract.
type RegistryRecommendedVersionRemovedIterator struct {
	Event *RegistryRecommendedVersionRemoved // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RegistryRecommendedVersionRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryRecommendedVersionRemoved)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RegistryRecommendedVersionRemoved)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RegistryRecommendedVersionRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryRecommendedVersionRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryRecommendedVersionRemoved represents a RecommendedVersionRemoved event raised by the Registry contract.
type RegistryRecommendedVersionRemoved struct {
	ContractName string
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterRecommendedVersionRemoved is a free log retrieval operation binding the contract event 0x07b20feb74e0118ee3c73d4cb8d0eb4da169604c68aa233293b094cedcd225f2.
//
// Solidity: event RecommendedVersionRemoved(string contractName)
func (_Registry *RegistryFilterer) FilterRecommendedVersionRemoved(opts *bind.FilterOpts) (*RegistryRecommendedVersionRemovedIterator, error) {

	logs, sub, err := _Registry.contract.FilterLogs(opts, "RecommendedVersionRemoved")
	if err != nil {
		return nil, err
	}
	return &RegistryRecommendedVersionRemovedIterator{contract: _Registry.contract, event: "RecommendedVersionRemoved", logs: logs, sub: sub}, nil
}

// WatchRecommendedVersionRemoved is a free log subscription operation binding the contract event 0x07b20feb74e0118ee3c73d4cb8d0eb4da169604c68aa233293b094cedcd225f2.
//
// Solidity: event RecommendedVersionRemoved(string contractName)
func (_Registry *RegistryFilterer) WatchRecommendedVersionRemoved(opts *bind.WatchOpts, sink chan<- *RegistryRecommendedVersionRemoved) (event.Subscription, error) {

	logs, sub, err := _Registry.contract.WatchLogs(opts, "RecommendedVersionRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryRecommendedVersionRemoved)
				if err := _Registry.contract.UnpackLog(event, "RecommendedVersionRemoved", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RegistryVersionAddedIterator is returned from FilterVersionAdded and is used to iterate over the raw logs and unpacked data for VersionAdded events raised by the Registry contract.
type RegistryVersionAddedIterator struct {
	Event *RegistryVersionAdded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RegistryVersionAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryVersionAdded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RegistryVersionAdded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RegistryVersionAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryVersionAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryVersionAdded represents a VersionAdded event raised by the Registry contract.
type RegistryVersionAdded struct {
	ContractName   string
	VersionName    string
	Implementation common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterVersionAdded is a free log retrieval operation binding the contract event 0x337b109e3f497728f2bdd27545c9ed1cb52ed4a4103cc94da88b868879c982e2.
//
// Solidity: event VersionAdded(string contractName, string versionName, address indexed implementation)
func (_Registry *RegistryFilterer) FilterVersionAdded(opts *bind.FilterOpts, implementation []common.Address) (*RegistryVersionAddedIterator, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _Registry.contract.FilterLogs(opts, "VersionAdded", implementationRule)
	if err != nil {
		return nil, err
	}
	return &RegistryVersionAddedIterator{contract: _Registry.contract, event: "VersionAdded", logs: logs, sub: sub}, nil
}

// WatchVersionAdded is a free log subscription operation binding the contract event 0x337b109e3f497728f2bdd27545c9ed1cb52ed4a4103cc94da88b868879c982e2.
//
// Solidity: event VersionAdded(string contractName, string versionName, address indexed implementation)
func (_Registry *RegistryFilterer) WatchVersionAdded(opts *bind.WatchOpts, sink chan<- *RegistryVersionAdded, implementation []common.Address) (event.Subscription, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _Registry.contract.WatchLogs(opts, "VersionAdded", implementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryVersionAdded)
				if err := _Registry.contract.UnpackLog(event, "VersionAdded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RegistryVersionRecommendedIterator is returned from FilterVersionRecommended and is used to iterate over the raw logs and unpacked data for VersionRecommended events raised by the Registry contract.
type RegistryVersionRecommendedIterator struct {
	Event *RegistryVersionRecommended // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RegistryVersionRecommendedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryVersionRecommended)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RegistryVersionRecommended)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RegistryVersionRecommendedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryVersionRecommendedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryVersionRecommended represents a VersionRecommended event raised by the Registry contract.
type RegistryVersionRecommended struct {
	ContractName string
	VersionName  string
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterVersionRecommended is a free log retrieval operation binding the contract event 0xb318550bf93edf51de4bae84db3deabd2a866cc407435a72317ca2503e2a07a6.
//
// Solidity: event VersionRecommended(string contractName, string versionName)
func (_Registry *RegistryFilterer) FilterVersionRecommended(opts *bind.FilterOpts) (*RegistryVersionRecommendedIterator, error) {

	logs, sub, err := _Registry.contract.FilterLogs(opts, "VersionRecommended")
	if err != nil {
		return nil, err
	}
	return &RegistryVersionRecommendedIterator{contract: _Registry.contract, event: "VersionRecommended", logs: logs, sub: sub}, nil
}

// WatchVersionRecommended is a free log subscription operation binding the contract event 0xb318550bf93edf51de4bae84db3deabd2a866cc407435a72317ca2503e2a07a6.
//
// Solidity: event VersionRecommended(string contractName, string versionName)
func (_Registry *RegistryFilterer) WatchVersionRecommended(opts *bind.WatchOpts, sink chan<- *RegistryVersionRecommended) (event.Subscription, error) {

	logs, sub, err := _Registry.contract.WatchLogs(opts, "VersionRecommended")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryVersionRecommended)
				if err := _Registry.contract.UnpackLog(event, "VersionRecommended", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RegistryVersionUpdatedIterator is returned from FilterVersionUpdated and is used to iterate over the raw logs and unpacked data for VersionUpdated events raised by the Registry contract.
type RegistryVersionUpdatedIterator struct {
	Event *RegistryVersionUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RegistryVersionUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryVersionUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RegistryVersionUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RegistryVersionUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryVersionUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryVersionUpdated represents a VersionUpdated event raised by the Registry contract.
type RegistryVersionUpdated struct {
	ContractName string
	VersionName  string
	Status       uint8
	BugLevel     uint8
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterVersionUpdated is a free log retrieval operation binding the contract event 0x0acf3e1a00b57bfc05ebf65957f42293847dc0938bfa1744660d6df56036d751.
//
// Solidity: event VersionUpdated(string contractName, string versionName, uint8 status, uint8 bugLevel)
func (_Registry *RegistryFilterer) FilterVersionUpdated(opts *bind.FilterOpts) (*RegistryVersionUpdatedIterator, error) {

	logs, sub, err := _Registry.contract.FilterLogs(opts, "VersionUpdated")
	if err != nil {
		return nil, err
	}
	return &RegistryVersionUpdatedIterator{contract: _Registry.contract, event: "VersionUpdated", logs: logs, sub: sub}, nil
}

// WatchVersionUpdated is a free log subscription operation binding the contract event 0x0acf3e1a00b57bfc05ebf65957f42293847dc0938bfa1744660d6df56036d751.
//
// Solidity: event VersionUpdated(string contractName, string versionName, uint8 status, uint8 bugLevel)
func (_Registry *RegistryFilterer) WatchVersionUpdated(opts *bind.WatchOpts, sink chan<- *RegistryVersionUpdated) (event.Subscription, error) {

	logs, sub, err := _Registry.contract.WatchLogs(opts, "VersionUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryVersionUpdated)
				if err := _Registry.contract.UnpackLog(event, "VersionUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}
