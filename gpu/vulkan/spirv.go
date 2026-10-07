package vulkan

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/bluescreen10/gamekit/gpu/internal/specialization"
)

// SPIR-V opcodes and the one decoration specConstants reads.
const (
	opName              = 5
	opTypeBool          = 20
	opTypeInt           = 21
	opTypeFloat         = 22
	opSpecConstantTrue  = 48
	opSpecConstantFalse = 49
	opSpecConstant      = 50
	opDecorate          = 71

	decorationSpecID = 1
)

// specConstants lists the specialization constants a SPIR-V module declares: each
// OpSpecConstant* decorated with a SpecId, its type, and its name where the module kept
// its debug names.
func specConstants(code []byte) ([]specialization.Declaration, error) {
	if len(code)%4 != 0 || len(code) < 20 {
		return nil, fmt.Errorf("SPIR-V is %d bytes, not a whole module", len(code))
	}
	words := make([]uint32, len(code)/4)
	binary.Read(bytes.NewReader(code), binary.LittleEndian, words)

	names := map[uint32]string{}
	specIDs := map[uint32]uint32{}
	types := map[uint32]specialization.Type{}
	var constants []specialization.Declaration
	for i := 5; i < len(words); {
		count := int(words[i] >> 16)
		if count == 0 || i+count > len(words) {
			return nil, fmt.Errorf("SPIR-V instruction at word %d overruns the module", i)
		}
		operands := words[i+1 : i+count]
		switch words[i] & 0xffff {
		case opName:
			names[operands[0]] = literalString(operands[1:])
		case opDecorate:
			if operands[1] == decorationSpecID {
				specIDs[operands[0]] = operands[2]
			}
		case opTypeBool:
			types[operands[0]] = specialization.Bool
		case opTypeInt:
			types[operands[0]] = specialization.Uint
			if operands[2] != 0 {
				types[operands[0]] = specialization.Int
			}
		case opTypeFloat:
			types[operands[0]] = specialization.Float
		case opSpecConstantTrue, opSpecConstantFalse, opSpecConstant:
			constants = append(constants, specialization.Declaration{ID: operands[1], Type: types[operands[0]]})
		}
		i += count
	}

	// A spec constant's SpecId and name are decorations of its result id, which the
	// list above held in place of an ID until every decoration had been read.
	var declared []specialization.Declaration
	for _, c := range constants {
		resultID := c.ID
		specID, ok := specIDs[resultID]
		if !ok {
			continue
		}
		declared = append(declared, specialization.Declaration{ID: specID, Name: names[resultID], Type: c.Type})
	}
	return declared, nil
}

// literalString decodes a SPIR-V literal string: UTF-8 packed into words, ending at
// its first NUL.
func literalString(words []uint32) string {
	data := make([]byte, 4*len(words))
	for i, w := range words {
		binary.LittleEndian.PutUint32(data[4*i:], w)
	}
	if end := bytes.IndexByte(data, 0); end >= 0 {
		data = data[:end]
	}
	return string(data)
}
