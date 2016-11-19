package mtree

// dhCreator is used in when building a DirectoryHierarchy
type dhCreator struct {
	DH       *DirectoryHierarchy
	operator operator
	curSet   *Entry
	curDir   *Entry
	curEnt   *Entry
}
