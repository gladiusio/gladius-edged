package networkd

type state struct {
  running bool
  content map[string]map[string]string
}
