package vm

import (
	"bytes"
	"github.com/dipperin/dipperin-core/common/vmcommon"
	"github.com/dipperin/dipperin-core/core/vm/common/utils"
	"github.com/dipperin/dipperin-core/core/vm/resolver"
	"github.com/dipperin/dipperin-core/third-party/life/exec"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/dipperin/dipperin-core/common"
	"github.com/dipperin/dipperin-core/common/math"
	"github.com/dipperin/dipperin-core/third-party/log"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
	"reflect"
	"strings"
)

var (
	errReturnInvalidRlpFormat   = errors.New("interpreter_life: invalid rlp format")
	errReturnInsufficientParams = errors.New("interpreter_life: invalid input. ele must greater than 2")
	errReturnInvalidAbi         = errors.New("interpreter_life: invalid abi, encoded fail")
	errReturnInputAbiNotMatch   = errors.New("interpreter_life: length of input and abi not match")
)

const (
	CALL_CONTRACT_FLAG = 9
)

type Interpreter interface {
	// Run loops and evaluates the contract's code with the given input data and returns
	// the return byte-slice and an error if one occurred.
	Run(vm *VM, contract *Contract, input []byte, create bool) ([]byte, error)
	// CanRun tells if the contract, passed as an argument, can be
	CanRun([]byte) bool
}

type WASMInterpreter struct {
	state    StateDB
	context  *Context
	config   exec.VMConfig
	resolver exec.ImportResolver
}

// NewWASMInterpreter returns a new instance of the Interpreter
func NewWASMInterpreter(state StateDB, context Context, vmConfig exec.VMConfig) *WASMInterpreter {
	return &WASMInterpreter{
		state,
		&context,
		vmConfig,
		&resolver.Resolver{},
	}
}

func (in *WASMInterpreter) Run(vm *VM, contract *Contract, input []byte, create bool) ([]byte, error) {
	if len(contract.Code) == 0 {
		log.Debug("Code Length is 0")
		return nil, nil
	}

	// 第一次rlp解析合约获取code
	contract.Input = input
	code, err := getCode(contract.Code)
	if err != nil {
		return nil, err
	}

	//　life方法注入新建虚拟机
	solver := resolver.NewResolver(vm, contract, in.state)
	lifeVm, err := exec.NewVirtualMachine(code, in.config, solver, nil)
	lifeVm.GasLimit = contract.Gas
	if err != nil {
		return []byte{}, err
	}
	defer func() {
		lifeVm.Stop()
	}()

	// 第二次rlp解析合约
	_, abi, code, initParams, err := parseRlpData(lifeVm, contract.Code)

	var (
		funcName   string
		txType     int
		params     []int64
		returnType string
	)

	if create {
		funcName = "init" // init function.
		params = initParams
	} else {
		// 通过ABI解析input
		txType, funcName, params, returnType, err = parseInputFromAbi(lifeVm, input, abi)
		if err != nil {
			/*			if err == errReturnInsufficientParams && txType == 0 { // transfer to contract address.
							return nil, nil
						}*/
			return nil, err
		}
		/*		if txType == 0 {
					return nil, nil
				}*/
	}
	log.Info("parseInput", "type", txType, "funcName", funcName, "params", params, "return", returnType, "err", err)

	//　获取entryID
	entryID, ok := lifeVm.GetFunctionExport(funcName)

	if !ok {
		return nil, fmt.Errorf("entryId not found")
	}

	res, err := lifeVm.Run(entryID, params...)
	log.Info("Run lifeVm", "Gas Used", lifeVm.GasUsed, "Gas", lifeVm.Gas, "Gas Limit", lifeVm.GasLimit)

	if err != nil {
		fmt.Println("throw exception:", err.Error())
		return nil, err
	}

	if contract.Gas > lifeVm.GasUsed {
		contract.Gas = contract.Gas - lifeVm.GasUsed
	} else {
		return nil, ErrOutOfGas
	}

	if create {
		return contract.Code, nil
	}

	switch returnType {
	case "void", "int8", "int", "int32", "int64":
		if txType == CALL_CONTRACT_FLAG {
			return vmcommon.Int64ToBytes(res), nil
		}
		bigRes := new(big.Int)
		bigRes.SetInt64(res)
		finalRes := vmcommon.Align32Bytes(math.U256(bigRes).Bytes())
		return finalRes, nil
	case "uint8", "uint16", "uint32", "uint64":
		if txType == CALL_CONTRACT_FLAG {
			return vmcommon.Uint64ToBytes(uint64(res)), nil
		}
		finalRes := vmcommon.Align32Bytes(vmcommon.Uint64ToBytes((uint64(res))))
		return finalRes, nil
	case "string":
		returnBytes := make([]byte, 0)
		copyData := lifeVm.Memory.Memory[res:]
		for _, v := range copyData {
			if v == 0 {
				break
			}
			returnBytes = append(returnBytes, v)
		}
		if txType == CALL_CONTRACT_FLAG {
			return returnBytes, nil
		}
		strHash := common.BytesToHash(vmcommon.Int32ToBytes(32))
		sizeHash := common.BytesToHash(vmcommon.Int64ToBytes(int64((len(returnBytes)))))
		var dataRealSize = len(returnBytes)
		if (dataRealSize % 32) != 0 {
			dataRealSize = dataRealSize + (32 - (dataRealSize % 32))
		}
		dataByt := make([]byte, dataRealSize)
		copy(dataByt[0:], returnBytes)

		finalData := make([]byte, 0)
		finalData = append(finalData, strHash.Bytes()...)
		finalData = append(finalData, sizeHash.Bytes()...)
		finalData = append(finalData, dataByt...)

		//fmt.Println("CallReturn:", string(returnBytes))
		return finalData, nil
	}
	return nil, nil
}

func (in *WASMInterpreter) CanRun([]byte) bool {
	return true
}

// input = RLP([txType][funcName][params])
// returnType[0] if more than 1 return
func parseInputFromAbi(vm *exec.VirtualMachine, input []byte, abi []byte) (txType int, funcName string, params []int64, returnType string, err error) {
	if input == nil || len(input) <= 1 {
		return -1, "", nil, "", fmt.Errorf("invalid input")
	}
	// [txType][funcName][args1][args2]
	// rlp decode
	ptr := new(interface{})
	err = rlp.Decode(bytes.NewReader(input), &ptr)
	if err != nil {
		return -1, "", nil, "", err
	}
	rlpList := reflect.ValueOf(ptr).Elem().Interface()

	if _, ok := rlpList.([]interface{}); !ok {
		return -1, "", nil, "", errReturnInvalidRlpFormat
	}

	iRlpList := rlpList.([]interface{})
	if len(iRlpList) < 2 {
		if len(iRlpList) != 0 {
			if v, ok := iRlpList[0].([]byte); ok {
				txType = int(vmcommon.BytesToInt64(v))
			}
		} else {
			txType = -1
		}
		return txType, "", nil, "", errReturnInsufficientParams
	}

	wasmAbi := new(utils.WasmAbi)
	err = wasmAbi.FromJson(abi)
	if err != nil {
		return -1, "", nil, "", errReturnInvalidAbi
	}

	params = make([]int64, 0)
	if v, ok := iRlpList[0].([]byte); ok {
		txType = int(vmcommon.BytesToInt64(v))
	}
	if v, ok := iRlpList[1].([]byte); ok {
		funcName = string(v)
	}

	var args []utils.InputParam
	for _, v := range wasmAbi.AbiArr {
		if strings.EqualFold(funcName, v.Name) && strings.EqualFold(v.Type, "function") {
			args = v.Inputs
			if len(v.Outputs) != 0 {
				returnType = v.Outputs[0].Type
			} else {
				returnType = "void"
			}
			break
		}
	}
	argsRlp := iRlpList[2:]
	if len(args) != len(argsRlp) {
		return -1, "", nil, returnType, errReturnInputAbiNotMatch
	}

	params = findParams(vm, args, argsRlp)
	return txType, funcName, params, returnType, nil
}

// rlpData=RLP([txType][code][abi][init params])
func parseRlpData(vm *exec.VirtualMachine, rlpData []byte) (int64, []byte, []byte, []int64, error) {
	ptr := new(interface{})
	err := rlp.Decode(bytes.NewReader(rlpData), &ptr)
	if err != nil {
		return -1, nil, nil, nil, err
	}
	rlpList := reflect.ValueOf(ptr).Elem().Interface()

	if _, ok := rlpList.([]interface{}); !ok {
		return -1, nil, nil, nil, errReturnInvalidRlpFormat
	}

	iRlpList := rlpList.([]interface{})
	if len(iRlpList) <= 2 {
		return -1, nil, nil, nil, fmt.Errorf("invalid input, ele must greater than 2")
	}
	var (
		txType int64
		code   []byte
		abi    []byte
	)
	if v, ok := iRlpList[0].([]byte); ok {
		txType = vmcommon.BytesToInt64(v)
	}
	if v, ok := iRlpList[1].([]byte); ok {
		code = v
	}
	if v, ok := iRlpList[2].([]byte); ok {
		abi = v
	}

	wasmAbi := new(utils.WasmAbi)
	err = wasmAbi.FromJson(abi)
	if err != nil {
		return -1, nil, nil, nil, errReturnInvalidAbi
	}

	var args []utils.InputParam
	for _, v := range wasmAbi.AbiArr {
		if strings.EqualFold("init", v.Name) && strings.EqualFold(v.Type, "function") {
			args = v.Inputs
			if len(v.Outputs) != 0 {
				return -1, nil, nil, nil, fmt.Errorf("invalid init function outputs length")
			}
			break
		}
	}
	argsRlp := iRlpList[3:]
	if len(args) != len(argsRlp) {
		return -1, nil, nil, nil, errReturnInputAbiNotMatch
	}
	params := findParams(vm, args, argsRlp)
	return txType, abi, code, params, nil
}

func getCode(rlpData []byte) (code []byte, err error) {
	ptr := new(interface{})
	err = rlp.Decode(bytes.NewReader(rlpData), &ptr)
	if err != nil {
		return nil, err
	}
	rlpList := reflect.ValueOf(ptr).Elem().Interface()

	if _, ok := rlpList.([]interface{}); !ok {
		return nil, errReturnInvalidRlpFormat
	}

	iRlpList := rlpList.([]interface{})
	if len(iRlpList) <= 2 {
		return nil, fmt.Errorf("invalid input, ele must greater than 2")
	}
	if v, ok := iRlpList[1].([]byte); ok {
		code = v
	}
	return code, nil
}

func findParams(vm *exec.VirtualMachine, args []utils.InputParam, argsRlp []interface{}) (params []int64) {
	// uint64 uint32  uint16 uint8 int64 int32  int16 int8 float32 float64 string void
	for i, v := range args {
		bts := argsRlp[i].([]byte)
		switch v.Type {
		case "string":
			pos := resolver.MallocString(vm, string(bts))
			params = append(params, pos)
		case "int8":
			params = append(params, int64(bts[0]))
		case "int16":
			params = append(params, int64(binary.BigEndian.Uint16(bts)))
		case "int32", "int":
			params = append(params, int64(binary.BigEndian.Uint32(bts)))
		case "int64":
			params = append(params, int64(binary.BigEndian.Uint64(bts)))
		case "uint8":
			params = append(params, int64(bts[0]))
		case "uint32", "uint":
			params = append(params, int64(binary.BigEndian.Uint32(bts)))
		case "uint64":
			params = append(params, int64(binary.BigEndian.Uint64(bts)))
		case "bool":
			params = append(params, int64(bts[0]))
		}
	}
	return
}
