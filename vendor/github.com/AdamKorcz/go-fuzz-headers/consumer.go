package gofuzzheaders

import (
    "errors"
    "fmt"
)

type ConsumeFuzzer struct {
    data            []byte
    CommandPart     []byte
    RestOfArray     []byte
    NumberOfCalls   int
}

func IsDivisibleBy(n int, divisibleby int) bool {
    return (n % divisibleby) == 0
}

func NewConsumer(fuzzData []byte) *ConsumeFuzzer {
    f := &ConsumeFuzzer{data: fuzzData}
    return f
}

/*
    SplitToSeveral splits the input into 3 chunks:
    1: the first byte - Is converted to an int, and
       that int determines the number of command-line
       calls the fuzzer will make.
    2: The next n bytes where n is equal to the int from
       the first byte. These n bytes are converted to
       a corresponding command and represent which
       commands will be called.
    3: The rest of the data array should have a length
       that is divisible by the number of calls.
       This part is split up into equally large chunks,
       and each chunk is used as parameters for the
       corresponding command.
*/
func (f *ConsumeFuzzer) Split(minCalls, maxCalls int) error {
    if len(f.data)==0 {
        fmt.Println("f.data is", f.data)
        return errors.New("Could not split")
    }
    numberOfCalls := int(f.data[0])
    if numberOfCalls < minCalls || numberOfCalls > maxCalls {
        return errors.New("Bad number of calls")

    }
    if len(f.data) < numberOfCalls+numberOfCalls+1 {
        return errors.New("Length of data does not match required parameters")
    }

    // Define part 2 and 3 of the data array
    commandPart := f.data[1 : numberOfCalls+1]
    restOfArray := f.data[numberOfCalls+1:]

    // Just a small check. It is necessary
    if len(commandPart) != numberOfCalls {
        return errors.New("Length of commandPart does not match number of calls")
    }

    // Check if restOfArray is divisible by numberOfCalls
    if !IsDivisibleBy(len(restOfArray), numberOfCalls) {
        return errors.New("Length of commandPart does not match number of calls")
    }
    f.CommandPart = commandPart
    f.RestOfArray = restOfArray
    f.NumberOfCalls = numberOfCalls
    return nil
}

