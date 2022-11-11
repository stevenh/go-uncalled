package uncalled

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_loadDefaultConfig(t *testing.T) {
	_, err := loadDefaultConfig()
	require.NoError(t, err)
}

func Test_quote(t *testing.T) {
	type args struct {
		s string
	}
	tests := map[string]struct {
		str  string
		want string
	}{
		"single-line": {
			str:  "my message",
			want: "`my message`",
		},
		"multi-line": {
			str:  "my message\nline two",
			want: `"my message\nline two"`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			res := quote(tt.str)
			require.Equal(t, tt.want, res)
		})
	}
}

func TestConfig_validate(t *testing.T) {
	defConfig, err := loadDefaultConfig()
	require.NoError(t, err)
	tests := map[string]struct {
		cfg Config
		err string
	}{
		"default": {
			cfg: *defConfig,
		},
		"bad-name": {
			cfg: Config{
				Rules: []Rule{
					{
						Name: "Bad Name",
					},
				},
			},
			err: `rule "Bad Name": contains non alpha numberic or uppercase charaters`,
		},
		"no-packages": {
			cfg: Config{
				Rules: []Rule{
					{
						Name: "my-rule",
					},
				},
			},
			err: `rule "my-rule": no packages`,
		},
		"no-call-results": {
			cfg: Config{
				Rules: []Rule{
					{
						Name:     "my-rule",
						Packages: []string{"context"},
					},
				},
			},
			err: `rule "my-rule": no call results`,
		},
		"multiple-expect-results": {
			cfg: Config{
				Rules: []Rule{
					{
						Name:     "my-rule",
						Packages: []string{"context"},
						Results: []*Result{
							{
								Type:   ".Context",
								Expect: &Expect{},
							},
							{
								Type:   ".Other",
								Expect: &Expect{},
							},
						},
					},
				},
			},
			err: `rule "my-rule": multiple results expecting a method`,
		},
		"no-expecting-results": {
			cfg: Config{
				Rules: []Rule{
					{
						Name:     "my-rule",
						Packages: []string{"context"},
						Results: []*Result{
							{
								Type: ".Context",
							},
						},
					},
				},
			},
			err: `rule "my-rule": no result expecting a method`,
		},
		"wildcard-expects": {
			cfg: Config{
				Rules: []Rule{
					{
						Name:     "my-rule",
						Packages: []string{"context"},
						Results: []*Result{
							{
								Type:   "_",
								Expect: &Expect{},
							},
						},
					},
				},
			},
			err: `rule "my-rule": result idx 0 is expected and wildcard`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.cfg.validate()
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}

			require.NoError(t, err)
		})
	}
}
