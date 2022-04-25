package cassandra

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"strings"
)

const (
	cassConfigTagName             = "cass-config"
	cassConfigTagOptionRecurse    = "recurse"
	cassConfigTagOptionRetainZero = "retainzero"
)

// cassConfigTag is the representation of a cass-config tag contents.
type cassConfigTag struct {
	paths      []cassConfigTagPath
	retainZero bool
	recurse    bool
}

func (t cassConfigTag) pathForVersion(version *semver.Version) *cassConfigTagPath {
	for _, path := range t.paths {
		if valid, _ := path.constraint.Validate(version); valid {
			return &path
		}
	}
	return nil
}

type cassConfigTagPath struct {
	constraint *semver.Constraints
	path       string
	segments   []string
}

func (p cassConfigTagPath) isInline() bool {
	return len(p.segments) == 0
}

// parseCassConfigTag parses a textual representation of a cass-config tag.
// The textual representation is of the following general form:
// tag := <path1>[,<path2>,...,<pathN>][;<option1>;<option2>;...;<optionN>]
// path := <constraint>:[<segments>]
// constraint := a valid semver constraint
// segments := a series of path segments separated by a slash, e.g. foo/bar/qix
// option := recurse | retainzero
func parseCassConfigTag(text string) (*cassConfigTag, error) {
	if len(text) == 0 {
		return nil, fmt.Errorf("empty %v tag", cassConfigTagName)
	}
	tag := &cassConfigTag{}
	parts := strings.Split(text, ";")
	if len(parts[0]) == 0 {
		return nil, fmt.Errorf("no path entry found in tag: '%v'", text)
	}
	entries := strings.Split(parts[0], ",")
	for _, entry := range entries {
		keyVal := strings.Split(entry, ":")
		if len(keyVal) != 2 {
			return nil, fmt.Errorf("wrong path entry: '%v'", entry)
		}
		path := cassConfigTagPath{}
		if constraint, err := semver.NewConstraint(strings.TrimSpace(keyVal[0])); err != nil {
			return nil, err
		} else {
			path.constraint = constraint
		}
		path.path = strings.Trim(keyVal[1], " /")
		if len(path.path) > 0 {
			path.segments = strings.Split(path.path, "/")
		}
		tag.paths = append(tag.paths, path)
	}
	for _, option := range parts[1:] {
		switch strings.TrimSpace(option) {
		case cassConfigTagOptionRecurse:
			tag.recurse = true
		case cassConfigTagOptionRetainZero:
			tag.retainZero = true
		default:
			return nil, fmt.Errorf("unknown option: '%v'", option)
		}
	}
	if tag.recurse && tag.retainZero {
		return nil, fmt.Errorf("tag cannot have both options %v and %v", cassConfigTagOptionRecurse, cassConfigTagOptionRetainZero)
	}
	if !tag.recurse {
		for _, path := range tag.paths {
			if len(path.segments) == 0 {
				return nil, errors.New("non-recursive field cannot have empty paths")
			}
		}
	}
	return tag, nil
}
