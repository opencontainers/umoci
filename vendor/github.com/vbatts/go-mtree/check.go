package mtree

// Check a root directory path against the DirectoryHierarchy, regarding only
// the available keywords from the list and each entry in the hierarchy.
// If keywords is nil, the check all present in the DirectoryHierarchy
//
// This is equivalent to creating a new DirectoryHierarchy with Walk(root, nil,
// keywords) and then doing a Compare(dh, newDh, keywords).
func Check(root string, dh *DirectoryHierarchy, keywords []string) ([]InodeDelta, error) {
	if keywords == nil {
		keywords = CollectUsedKeywords(dh)
	}

	newDh, err := Walk(root, nil, keywords)
	if err != nil {
		return nil, err
	}

	// TODO: Handle tar_time, if necessary.
	return Compare(dh, newDh, keywords)
}

// TarCheck is the tar equivalent of checking a file hierarchy spec against a
// tar stream to determine if files have been changed. This is precisely
// equivalent to Compare(dh, tarDH, keywords).
func TarCheck(tarDH, dh *DirectoryHierarchy, keywords []string) ([]InodeDelta, error) {
	if keywords == nil {
		keywords = CollectUsedKeywords(dh)
	}
	return Compare(dh, tarDH, keywords)
}
