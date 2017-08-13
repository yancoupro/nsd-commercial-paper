package main


import (
	"fmt"
	"encoding/json"
	"time"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

var logger = shim.NewLogger("BookChaincode")

const indexName = `Book`

// BookChaincode
type BookChaincode struct {
}

type BookValue struct {
	Quantity   		int 	`json:"quantity"`
}

//TODO move to common package
type Balance struct {
	Account 		string 	`json:"account"`
	Division 		string 	`json:"division"`
}

//TODO reuse Position struct
type Book struct {
	Balance 		Balance `json:"balance"`
	Security        string 	`json:"security"`
	Quantity   		int 	`json:"quantity"`
}

//TODO move to common package
type Instruction struct {
	Transferer      Balance `json:"transferer"`
	Receiver        Balance `json:"receiver"`
	Security        string 	`json:"security"`
	Quantity        string 	`json:"quantity"`
	Reference       string 	`json:"reference"`
	InstructionDate string 	`json:"instructionDate"`
	TradeDate       string 	`json:"tradeDate"`
	DeponentFrom    string  `json:"deponentFrom"`
	DeponentTo      string  `json:"deponentTo"`
	Status          string 	`json:"status"`
	Initiator       string 	`json:"initiator"`
	Reason          Reason 	`json:"reason"`
}

type Reason struct {
	Document 		string 	`json:"document"`
	Description 	string 	`json:"description"`
	DocumentDate 	string 	`json:"documentDate"`
}

type KeyModificationValue struct {
	TxId      string 			`json:"txId"`
	Value     BookValue  		`json:"value"`
	Timestamp string 			`json:"timestamp"`
	IsDelete  bool   			`json:"isDelete"`
}

func (t *BookChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response  {
	logger.Info("########### BookChaincode Init ###########")

	_, args := stub.GetFunctionAndParameters()

	return t.put(stub, args)
}

func (t *BookChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	logger.Info("########### BookChaincode Invoke ###########")

	function, args := stub.GetFunctionAndParameters()

	if function == "put" {
		return t.put(stub, args)
	}
	if function == "move" {
		return t.move(stub, args)
	}
	if function == "check" {
		return t.check(stub, args)
	}
	if function == "query" {
		return t.query(stub, args)
	}
	if function == "history" {
		return t.history(stub, args)
	}

	err := fmt.Sprintf("Unknown function, check the first argument, must be one of: " +
		"put, move, check, query, history. But got: %v", args[0])
	logger.Error(err)
	return shim.Error(err)
}

func (t *BookChaincode) put(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	// account, division, security, quantity
	if len(args) != 4 {
		return shim.Error("Incorrect number of arguments. " +
			"Expecting account, division, security, quantity")
	}

	account := args[0]
	division := args[1]
	security := args[2]
	quantity, err := strconv.Atoi(args[3])
	if err != nil {
		return shim.Error(err.Error())
	}

	// account-division-security
	key, err := stub.CreateCompositeKey(indexName, []string{account, division, security})
	if err != nil {
		return shim.Error(err.Error())
	}

	value, err := json.Marshal(BookValue{Quantity: quantity})
	if err != nil {
		return shim.Error(err.Error())
	}

	err = stub.PutState(key, value)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

func (t *BookChaincode) check(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	// account, division, security, quantity
	if len(args) != 4 {
		return pb.Response{Status:400, Message: "Incorrect number of arguments. " +
			"Expecting account, division, security, quantity"}
	}

	account := args[0]
	division := args[1]
	security := args[2]
	quantity, err := strconv.Atoi(args[3])
	if err != nil {
		return shim.Error(err.Error())
	}

	keyFrom, err := stub.CreateCompositeKey(indexName, []string{account, division, security})
	if err != nil {
		return shim.Error(err.Error())
	}

	bytes, err := stub.GetState(keyFrom)
	if err != nil {
		return shim.Error(err.Error())
	}

	if bytes == nil {
		return pb.Response{Status:404, Message: "cannot find position"}
	}

	var value BookValue
	err = json.Unmarshal(bytes, &value)
	if err != nil {
		return shim.Error(err.Error())
	}

	if value.Quantity < quantity {
		return pb.Response{Status:409, Message: "quantity less than current balance"}
	}

	return shim.Success(nil)
}

func (t *BookChaincode) move(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	// accountFrom, divisionFrom, accountTo, divisionTo, security, quantity, reference, instructionDate, tradeDate
	if len(args) < 6 {
		return pb.Response{Status:400, Message: fmt.Sprintf("Incorrect number of arguments. " +
			"Expecting accountFrom, divisionFrom, accountTo, divisionTo, security, quantity. " +
			"But got %d args: %s", len(args), args)}
	}

	accountFrom := args[0]
	divisionFrom := args[1]
	accountTo := args[2]
	divisionTo := args[3]
	security := args[4]
	quantity, err := strconv.Atoi(args[5])
	if err != nil {
		return shim.Error(err.Error())
	}

	var instruction Instruction
	if len(args) == 9 {
		instruction = Instruction{Transferer:Balance{Account:accountFrom, Division:divisionFrom},
			Receiver:Balance{Account:accountTo, Division:divisionTo},
			Security:security,
			Quantity:strconv.Itoa(quantity),
			Reference:args[6],
			InstructionDate:args[7],
			TradeDate:args[8],}

		//TODO check stored list for this instructions has been executed already
	}

	keyFrom, err := stub.CreateCompositeKey(indexName, []string{accountFrom, divisionFrom, security})
	if err != nil {
		return shim.Error(err.Error())
	}

	bytes, err := stub.GetState(keyFrom)
	if err != nil {
		return shim.Error(err.Error())
	}

	if bytes == nil {
		if instruction != (Instruction{}) {
			instruction.Status = "declined"
			err = setInstructionEvent(stub, instruction)
			if err != nil {
				return shim.Error(err.Error())
			}
		}

		return pb.Response{Status:404, Message: "cannot find position"}
	}

	var valueFrom BookValue
	err = json.Unmarshal(bytes, &valueFrom)
	if err != nil {
		return shim.Error(err.Error())
	}

	if valueFrom.Quantity < quantity {
		if instruction != (Instruction{}) {
			instruction.Status = "declined"
			err = setInstructionEvent(stub, instruction)
			if err != nil {
				return shim.Error(err.Error())
			}
		}

		return pb.Response{Status:409, Message: "cannot move quantity less than current balance"}
	}

	valueFrom.Quantity = valueFrom.Quantity - quantity

	newBytes, err := json.Marshal(valueFrom)
	if err != nil {
		return shim.Error(err.Error())
	}

	err = stub.PutState(keyFrom, newBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	keyTo, err := stub.CreateCompositeKey(indexName, []string{accountTo, divisionTo, security})
	if err != nil {
		return shim.Error(err.Error())
	}

	bytes, err = stub.GetState(keyTo)
	if err != nil {
		return shim.Error(err.Error())
	}

	if bytes == nil {
		newBytes, err = json.Marshal(BookValue{Quantity: quantity})
		if err != nil {
			return shim.Error(err.Error())
		}
	} else {
		var valueTo BookValue
		err = json.Unmarshal(bytes, &valueTo)
		if err != nil {
			return shim.Error(err.Error())
		}

		valueTo.Quantity = valueTo.Quantity + quantity

		newBytes, err = json.Marshal(valueTo)
		if err != nil {
			return shim.Error(err.Error())
		}
	}

	err = stub.PutState(keyTo, newBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	if instruction != (Instruction{}) {
		instruction.Status = "executed"
		err = setInstructionEvent(stub, instruction)
		if err != nil {
			return shim.Error(err.Error())
		}

		//TODO save to the ledger list of executed instructions
	}

	return shim.Success(nil)
}

func setInstructionEvent(stub shim.ChaincodeStubInterface, instruction Instruction) (error) {
	logger.Debugf("setInstructionEvent", instruction)

	bytes, err := json.Marshal(instruction)
	if err != nil {
		return err
	}

	err = stub.SetEvent("Instruction." + instruction.Status, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (t *BookChaincode) query(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	it, err := stub.GetStateByPartialCompositeKey(indexName, []string{})
	if err != nil {
		return shim.Error(err.Error())
	}
	defer it.Close()

	books := []Book{}
	for it.HasNext() {
		responseRange, err := it.Next()
		if err != nil {
			return shim.Error(err.Error())
		}

		//account-division-security
		_, compositeKeyParts, err := stub.SplitCompositeKey(responseRange.Key)
		if err != nil {
			return shim.Error(err.Error())
		}

		var value BookValue
		err = json.Unmarshal(responseRange.Value, &value)
		if err != nil {
			return shim.Error(err.Error())
		}

		book := Book {
			Balance: Balance {
				Account: compositeKeyParts[0],
				Division: compositeKeyParts[1],
			},
			Security: compositeKeyParts[2],
			Quantity: value.Quantity,
		}

		books = append(books, book)
	}

	result, err := json.Marshal(books)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(result)
}

func (t *BookChaincode) history(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 3 {
		return pb.Response{Status:400, Message: "Incorrect number of arguments. " +
			"Expecting account, division, security"}
	}

	//account-division-security
	key, err := stub.CreateCompositeKey(indexName, args)
	if err != nil {
		return shim.Error(err.Error())
	}

	it, err := stub.GetHistoryForKey(key)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer it.Close()

	modifications := []KeyModificationValue{}

	for it.HasNext() {
		response, err := it.Next()
		if err != nil {
			return shim.Error(err.Error())
		}

		var entry KeyModificationValue

		entry.TxId = response.GetTxId()
		entry.IsDelete = response.GetIsDelete()
		ts := response.GetTimestamp()

		if ts != nil {
			entry.Timestamp = time.Unix(ts.Seconds, int64(ts.Nanos)).String()
		}

		err = json.Unmarshal(response.GetValue(), &entry.Value)
		if err != nil {
			return shim.Error(err.Error())
		}

		modifications = append(modifications, entry)
	}

	result, err := json.Marshal(modifications)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(result)
}

func main() {
	err := shim.Start(new(BookChaincode))
	if err != nil {
		logger.Errorf("Error starting Book chaincode: %s", err)
	}
}

