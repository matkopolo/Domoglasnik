package analytics

type buildingPartGroup struct {
	name      string
	isPrimary bool
}

var buildingPartGroups = map[int]buildingPartGroup{
	1:  {"house", true},
	2:  {"apartment", true},
	3:  {"garage", false},
	4:  {"garage", false},
	5:  {"commercial", true},
	6:  {"commercial", true},
	7:  {"commercial", true},
	8:  {"commercial", true},
	9:  {"commercial", true},
	10: {"commercial", true},
	11: {"commercial", true},
	12: {"commercial", true},
	13: {"commercial", true},
	14: {"unknown", false},
	15: {"unknown", false},
}

// ClassifySalePackage groups all components of one legal transaction. Land and
// parking are auxiliary when the package contains exactly one primary use.
func ClassifySalePackage(buildingPartTypes, landTypes []int) string {
	primary := make(map[string]struct{})
	hasGarage := false
	for _, sourceCode := range buildingPartTypes {
		group, known := buildingPartGroups[sourceCode]
		if !known {
			continue
		}
		if group.isPrimary {
			primary[group.name] = struct{}{}
		}
		hasGarage = hasGarage || group.name == "garage"
	}
	if len(primary) > 1 {
		return "mixed"
	}
	for group := range primary {
		return group
	}
	if hasGarage {
		return "garage"
	}
	if len(landTypes) > 0 {
		return "land"
	}
	return "unknown"
}

var rentalSpaceGroups = map[int]string{
	1: "house", 2: "apartment", 3: "garage", 4: "garage",
	5: "commercial", 6: "commercial", 7: "commercial", 8: "commercial",
	9: "commercial", 10: "commercial", 11: "commercial", 12: "commercial",
	13: "commercial", 14: "unknown", 15: "unknown", 16: "room",
}

// ClassifyRentalPackage only accepts a homogeneous use. Unlike a sale package,
// an apartment plus garage is mixed because a total monthly rent cannot be
// safely separated between the rented uses.
func ClassifyRentalPackage(spaceTypes []int) string {
	groups := make(map[string]struct{})
	for _, sourceCode := range spaceTypes {
		group, known := rentalSpaceGroups[sourceCode]
		if known && group != "unknown" {
			groups[group] = struct{}{}
		}
	}
	if len(groups) > 1 {
		return "mixed"
	}
	for group := range groups {
		return group
	}
	return "unknown"
}
