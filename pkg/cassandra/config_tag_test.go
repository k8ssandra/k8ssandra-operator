package cassandra

import (
	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_parseCassConfigTag(t *testing.T) {
	newConstraint := func(text string) *semver.Constraints {
		c, _ := semver.NewConstraint(text)
		return c
	}
	tests := []struct {
		name    string
		text    string
		want    *cassConfigTag
		wantErr assert.ErrorAssertionFunc
	}{
		{
			"simple",
			"*:foo/bar",
			&cassConfigTag{
				paths: []cassConfigTagPath{{
					constraint: newConstraint("*"),
					path:       "foo/bar",
					segments:   []string{"foo", "bar"},
				}},
			},
			assert.NoError,
		},
		{
			"simple recurse",
			"*:foo/bar;recurse",
			&cassConfigTag{
				paths: []cassConfigTagPath{{
					constraint: newConstraint("*"),
					path:       "foo/bar",
					segments:   []string{"foo", "bar"},
				}},
				recurse: true,
			},
			assert.NoError,
		},
		{
			"simple retainzero",
			"*:foo/bar;retainzero",
			&cassConfigTag{
				paths: []cassConfigTagPath{{
					constraint: newConstraint("*"),
					path:       "foo/bar",
					segments:   []string{"foo", "bar"},
				}},
				retainZero: true,
			},
			assert.NoError,
		},
		{
			"many constraints",
			"^3.11.x:foo/bar,>=4.x:foo/qix",
			&cassConfigTag{
				paths: []cassConfigTagPath{
					{
						constraint: newConstraint("^3.11.x"),
						path:       "foo/bar",
						segments:   []string{"foo", "bar"},
					},
					{
						constraint: newConstraint(">=4.x"),
						path:       "foo/qix",
						segments:   []string{"foo", "qix"},
					},
				},
			},
			assert.NoError,
		},
		{
			"many constraints whitespace",
			"^ 3.11.x : /foo/bar/ , >=4.x : foo/qix ; recurse ",
			&cassConfigTag{
				paths: []cassConfigTagPath{
					{
						constraint: newConstraint("^3.11.x"),
						path:       "foo/bar",
						segments:   []string{"foo", "bar"},
					},
					{
						constraint: newConstraint(">=4.x"),
						path:       "foo/qix",
						segments:   []string{"foo", "qix"},
					},
				},
				recurse: true,
			},
			assert.NoError,
		},
		{
			"no path recursive",
			"^3.11.x: , >=4.x: ;recurse",
			&cassConfigTag{
				paths: []cassConfigTagPath{
					{
						constraint: newConstraint("^3.11.x"),
						path:       "",
						segments:   nil,
					},
					{
						constraint: newConstraint(">=4.x"),
						path:       "",
						segments:   nil,
					},
				},
				recurse: true,
			},
			assert.NoError,
		},
		{
			"root path recursive",
			"*:/;recurse",
			&cassConfigTag{
				paths: []cassConfigTagPath{
					{
						constraint: newConstraint("*"),
						path:       "",
						segments:   nil,
					},
				},
				recurse: true,
			},
			assert.NoError,
		},
		{
			"trim whitespace and slash",
			"*: /foo/bar/qix/ ",
			&cassConfigTag{
				paths: []cassConfigTagPath{
					{
						constraint: newConstraint("*"),
						path:       "foo/bar/qix",
						segments:   []string{"foo", "bar", "qix"},
					},
				},
			},
			assert.NoError,
		},
		{
			"empty tag",
			"",
			nil,
			func(t assert.TestingT, err error, mesAndArgs ...interface{}) bool {
				return assert.Equal(t, "empty cass-config tag", err.Error())
			},
		},
		{
			"no path entry",
			";retainzero",
			nil,
			func(t assert.TestingT, err error, mesAndArgs ...interface{}) bool {
				return assert.Equal(t, "no path entry found in tag: ';retainzero'", err.Error())
			},
		},
		{
			"wrong path entry",
			"wrong",
			nil,
			func(t assert.TestingT, err error, mesAndArgs ...interface{}) bool {
				return assert.Equal(t, "wrong path entry: 'wrong'", err.Error())
			},
		},
		{
			"wrong constraint",
			"wrong:foo/bar",
			nil,
			func(t assert.TestingT, err error, mesAndArgs ...interface{}) bool {
				return assert.Equal(t, "improper constraint: wrong", err.Error())
			},
		},
		{
			"unknown option",
			"*:foo/bar;wrong",
			nil,
			func(t assert.TestingT, err error, mesAndArgs ...interface{}) bool {
				return assert.Equal(t, "unknown option: 'wrong'", err.Error())
			},
		},
		{
			"recurse and retainzero",
			"*:foo/bar;recurse;retainzero",
			nil,
			func(t assert.TestingT, err error, mesAndArgs ...interface{}) bool {
				return assert.Contains(t, err.Error(), "tag cannot have both options")
			},
		},
		{
			"non recurse and empty path",
			"*:/",
			nil,
			func(t assert.TestingT, err error, mesAndArgs ...interface{}) bool {
				return assert.Contains(t, err.Error(), "non-recursive field cannot have empty paths")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCassConfigTag(tt.text)
			assert.Equal(t, tt.want, got)
			tt.wantErr(t, err)
		})
	}
}
