package hydra

func (self *Application) getBuiltinModules() []*Module {
	return []*Module{
		{
			Name:      `Hydra`,
			Singleton: true,
			Imports: []string{
				`QtQuick 2.0`,
				`QtQuick.Window 2.0`,
			},
			Definition: &Component{
				Type: `Item`,
				ID:   `hydra`,
				Public: []*Property{
					{
						Type:  `Item`,
						Name:  `paths`,
						Value: Literal(`i_paths`),
					}, {
						Type: `var`,
						Name: `root`,
					},
				},
				Components: []*Component{
					{
						Type: `Item`,
						ID:   `i_paths`,
						Functions: []Function{
							{
								Name:       `basename`,
								Arguments:  []string{`path`},
								Definition: `return path.replace(/\\/g,'/').replace( /.*\//, '')`,
							}, {
								Name:       `dirname`,
								Arguments:  []string{`path`},
								Definition: `return path.replace(/\\/g, '/').replace(/\/?[^\/]*$/, '')`,
							},
						},
					},
				},
				Functions: []Function{
					{
						Name:       `vw`,
						Arguments:  []string{`pct`, `parent`},
						Definition: `return (parent || root || Screen).width * parseFloat(pct / 100.0);`,
					}, {
						Name:       `vh`,
						Arguments:  []string{`pct`, `parent`},
						Definition: `return (parent || root || Screen).height * parseFloat(pct / 100.0);`,
					}, {
						Name:       `vmin`,
						Arguments:  []string{`pct`, `parent`},
						Definition: `return Math.min(vw(pct, parent), vh(pct, parent));`,
					}, {
						Name:       `vmax`,
						Arguments:  []string{`pct`, `parent`},
						Definition: `return Math.max(vw(pct, parent), vh(pct, parent));`,
					},
				},
			},
		},
	}
}
