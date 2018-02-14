package types

// TODO: This is temporary, should be moved once we think about the easier chain config

var (
	// AuthorizedAddresses specifies if the chain should only allow (and support) authorized addresses
	AuthorizedAddresses = true
	// InitialAddresses are the initial authorized addresses for the chain. All addresses defined in the
	// genesis block must be here as well
	InitialAddresses = []UnlockHash{loadInitialAddress()}
)

func loadInitialAddress() UnlockHash {
	uh := UnlockHash{}
	uh.LoadString("e66bbe9638ae0e998641dc9faa0180c15a1071b1767784cdda11ad3c1d309fa692667931be66")
	return uh
}
