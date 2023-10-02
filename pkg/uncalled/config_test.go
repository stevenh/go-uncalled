package uncalled

import (
	_ "embed"
	"sync"
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
			err: `rule "Bad Name": contains non alpha numeric or uppercase characters`,
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

type copyRes struct {
	cfg *Config
	err error
}

func TestConfig_copy(t *testing.T) {
	cfg, err := loadDefaultConfig()
	require.NoError(t, err)

	want, err := cfg.yaml()
	require.NoError(t, err)

	tests := map[string]struct {
		cfg     Config
		max     int
		wantErr bool
	}{
		"race": {
			cfg: *cfg,
			max: 10,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.cfg.validate()
			require.NoError(t, err)

			results := make(chan copyRes, tt.max)
			hold := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(tt.max)
			for i := 0; i < tt.max; i++ {
				go func() {
					defer wg.Done()
					<-hold // Wait to unblock to maximise chance of race.
					cfg, err := tt.cfg.copy()
					results <- copyRes{cfg: cfg, err: err}
				}()
			}
			go func() {
				close(hold)
				wg.Wait()
				close(results)
			}()
			for r := range results {
				require.NoError(t, r.err)

				buf, err := r.cfg.yaml()
				require.NoError(t, err)
				require.Equal(t, want, buf)
			}
		})
	}
}
