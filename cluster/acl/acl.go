package acl

// ACL represents allowed traffic to a machine.
type ACL struct {
	CidrIP  string
	MinPort int
	MaxPort int
}

// Slice is an alias for []ACL to allow for joins
type Slice []ACL

// Get returns the value contained at the given index
func (slc Slice) Get(ii int) interface{} {
	return slc[ii]
}

// Len returns the number of items in the slice
func (slc Slice) Len() int {
	return len(slc)
}
