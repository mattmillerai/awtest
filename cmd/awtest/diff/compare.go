package diff

import "sort"

// Call status values for a (serviceName, methodName) pair.
const (
	StatusOK      = "ok"
	StatusError   = "error"
	StatusPartial = "partial"
	StatusNone    = "none"
)

// Call change classifications.
const (
	ChangeNewlyAllowed = "newly_allowed"
	ChangeNewlyDenied  = "newly_denied"
	ChangeChanged      = "changed"
)

// Resource identifies a resource found by a call.
type Resource struct {
	ServiceName  string `json:"serviceName"`
	MethodName   string `json:"methodName"`
	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
}

// CallChange describes a call whose status changed in a way that is not
// already visible as an added or removed resource.
type CallChange struct {
	ServiceName string `json:"serviceName"`
	MethodName  string `json:"methodName"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Change      string `json:"change"`
}

// Report is the result of comparing two scans.
type Report struct {
	Added   []Resource
	Removed []Resource
	Changed []CallChange
}

// Identical reports whether the two scans have no differences.
func (r Report) Identical() bool {
	return len(r.Added) == 0 && len(r.Removed) == 0 && len(r.Changed) == 0
}

type callKey struct{ service, method string }

type index struct {
	types  map[Resource]string // identity (ResourceType blank) -> resource type
	hasOK  map[callKey]bool
	hasErr map[callKey]bool
}

func buildIndex(entries []Entry) index {
	ix := index{
		types:  map[Resource]string{},
		hasOK:  map[callKey]bool{},
		hasErr: map[callKey]bool{},
	}
	for _, e := range entries {
		ck := callKey{e.ServiceName, e.MethodName}
		if e.Error != "" {
			ix.hasErr[ck] = true
			continue
		}
		ix.hasOK[ck] = true
		id := Resource{ServiceName: e.ServiceName, MethodName: e.MethodName, ResourceName: e.ResourceName}
		if _, seen := ix.types[id]; !seen {
			ix.types[id] = e.ResourceType
		}
	}
	return ix
}

func (ix index) status(ck callKey) string {
	switch {
	case ix.hasOK[ck] && ix.hasErr[ck]:
		return StatusPartial
	case ix.hasOK[ck]:
		return StatusOK
	case ix.hasErr[ck]:
		return StatusError
	}
	return StatusNone
}

// Compare computes the differences between two scans. Output is sorted by
// service, method, then resource name.
func Compare(before, after []Entry) Report {
	b, a := buildIndex(before), buildIndex(after)
	rep := Report{Added: []Resource{}, Removed: []Resource{}, Changed: []CallChange{}}

	for id, typ := range a.types {
		if _, ok := b.types[id]; !ok {
			id.ResourceType = typ
			rep.Added = append(rep.Added, id)
		}
	}
	for id, typ := range b.types {
		if _, ok := a.types[id]; !ok {
			id.ResourceType = typ
			rep.Removed = append(rep.Removed, id)
		}
	}

	calls := map[callKey]struct{}{}
	for _, ix := range []index{b, a} {
		for ck := range ix.hasOK {
			calls[ck] = struct{}{}
		}
		for ck := range ix.hasErr {
			calls[ck] = struct{}{}
		}
	}
	for ck := range calls {
		bs, as := b.status(ck), a.status(ck)
		if bs == as {
			continue
		}
		if bs != StatusError && bs != StatusPartial && as != StatusError && as != StatusPartial {
			continue
		}
		rep.Changed = append(rep.Changed, CallChange{
			ServiceName: ck.service, MethodName: ck.method,
			Before: bs, After: as, Change: classify(bs, as),
		})
	}

	sortResources(rep.Added)
	sortResources(rep.Removed)
	sort.Slice(rep.Changed, func(i, j int) bool {
		x, y := rep.Changed[i], rep.Changed[j]
		if x.ServiceName != y.ServiceName {
			return x.ServiceName < y.ServiceName
		}
		return x.MethodName < y.MethodName
	})
	return rep
}

func classify(before, after string) string {
	switch {
	case before == StatusError && (after == StatusOK || after == StatusPartial):
		return ChangeNewlyAllowed
	case after == StatusError:
		return ChangeNewlyDenied
	}
	return ChangeChanged
}

func sortResources(rs []Resource) {
	sort.Slice(rs, func(i, j int) bool {
		x, y := rs[i], rs[j]
		if x.ServiceName != y.ServiceName {
			return x.ServiceName < y.ServiceName
		}
		if x.MethodName != y.MethodName {
			return x.MethodName < y.MethodName
		}
		return x.ResourceName < y.ResourceName
	})
}
