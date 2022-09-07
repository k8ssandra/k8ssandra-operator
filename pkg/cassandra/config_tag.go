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
	paths      map[string][]cassConfigTagPath
	retainZero bool
	recurse    bool
}

func (t cassConfigTag) pathForVersion(version *semver.Version, serverType string) *cassConfigTagPath {
	for _, path := range t.paths[strings.ToLower(serverType)] {
		if valid, _ := path.constraint.Validate(version); valid {
			return &path
		}
	}
	for _, path := range t.paths["*"] {
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
// tag := <path1>[;<path2>;...;<pathN>][;<option1>;<option2>;...;<optionN>]
// path := <typeAndConstraint1>[,<typeAndConstraint2>,...,<typeAndConstraintN>]:[<segments>]
// typeAndConstraint := [<type>@]<constraint>
// type := any string (defaults to cassandra if not present)
// constraint := a valid semver constraint
// segments := a series of path segments separated by a slash, e.g. foo/bar/qix
// option := recurse | retainzero
func parseCassConfigTag(text string) (*cassConfigTag, error) {
	if len(strings.TrimSpace(text)) == 0 {
		return nil, fmt.Errorf("empty %v tag", cassConfigTagName)
	}
	tag := &cassConfigTag{}
	parts := strings.Split(text, ";")
	for _, part := range parts {
		switch strings.TrimSpace(part) {
		case cassConfigTagOptionRecurse:
			tag.recurse = true
		case cassConfigTagOptionRetainZero:
			tag.retainZero = true
		default:
			if err := parsePath(tag, part); err != nil {
				return nil, err
			}
		}
	}
	if tag.recurse && tag.retainZero {
		return nil, fmt.Errorf("tag cannot have both options %v and %v", cassConfigTagOptionRecurse, cassConfigTagOptionRetainZero)
	}
	if !tag.recurse {
		for _, paths := range tag.paths {
			for _, path := range paths {
				if len(path.segments) == 0 {
					return nil, errors.New("non-recursive field cannot have empty paths")
				}
			}
		}
	}
	return tag, nil
}

func parsePath(tag *cassConfigTag, part string) error {
	var constraints = make(map[string]string)
	var segmentsPart string
	pathParts := strings.Split(part, ":")
	if len(pathParts) == 1 {
		constraints["*"] = "*"
		segmentsPart = pathParts[0]
	} else if len(pathParts) == 2 {
		typeAndConstraintsParts := strings.Split(pathParts[0], ",")
		for _, typeAndConstraintPart := range typeAndConstraintsParts {
			typeAndConstraint := strings.Split(typeAndConstraintPart, "@")
			var serverType, constraintStr string
			if len(typeAndConstraint) == 1 {
				serverType = "cassandra"
				constraintStr = strings.TrimSpace(typeAndConstraint[0])
			} else if len(typeAndConstraint) == 2 {
				serverType = strings.ToLower(strings.TrimSpace(typeAndConstraint[0]))
				constraintStr = strings.TrimSpace(typeAndConstraint[1])
			} else {
				return fmt.Errorf("wrong type and constraint: '%v'", typeAndConstraintPart)
			}
			constraints[serverType] = constraintStr
		}
		segmentsPart = pathParts[1]
	} else {
		return fmt.Errorf("wrong path entry: '%v'", part)
	}
	path := cassConfigTagPath{}
	path.path = strings.Trim(segmentsPart, " /")
	if len(path.path) > 0 {
		path.segments = strings.Split(path.path, "/")
	}
	if tag.paths == nil {
		tag.paths = make(map[string][]cassConfigTagPath)
	}
	for serverType, constraint := range constraints {
		constr, err := semver.NewConstraint(constraint)
		if err != nil {
			return err
		}
		path.constraint = constr
		if tag.paths[serverType] == nil {
			tag.paths[serverType] = []cassConfigTagPath{}
		}
		tag.paths[serverType] = append(tag.paths[serverType], path)
	}
	return nil
}
