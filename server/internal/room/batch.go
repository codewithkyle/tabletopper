package room

import "context"

type Batch struct {
	Commands []Command
}

func (c *Batch) Authorize(s *State, a Actor) error {
	for _, cmd := range c.Commands {
		if err := cmd.Authorize(s, a); err != nil {
			return err
		}
	}
	return nil
}
func (c *Batch) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	var out []Signal
	for _, cmd := range c.Commands {
		sigs, err := cmd.Apply(s, a, env)
		if err != nil {
			return nil, err
		}
		out = append(out, sigs...)
	}
	return out, nil
}
func (c *Batch) Resolve(ctx context.Context, lib Library, s *State) error {
	for _, cmd := range c.Commands {
		resolver, ok := cmd.(Resolver)
		if !ok {
			continue
		}
		if err := resolver.Resolve(ctx, lib, s); err != nil {
			return err
		}
	}
	return nil
}
