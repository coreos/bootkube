package block

import (
	"github.com/Microsoft/azure-vhd-utils-for-go/vhdcore/bat"
	"github.com/Microsoft/azure-vhd-utils-for-go/vhdcore/footer"
	"github.com/Microsoft/azure-vhd-utils-for-go/vhdcore/header"
	"github.com/Microsoft/azure-vhd-utils-for-go/vhdcore/reader"
)

// FactoryParams represents type of the parameter for different disk block
// factories.
//
type FactoryParams struct {
	VhdFooter            *footer.Footer
	VhdHeader            *header.Header
	BlockAllocationTable *bat.BlockAllocationTable
	VhdReader            *reader.VhdReader
	ParentBlockFactory   Factory
}
