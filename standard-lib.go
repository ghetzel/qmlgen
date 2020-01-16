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
						Type: `var`,
						Name: `root`,
					}, {
						Type:  `Item`,
						Name:  `paths`,
						Value: Literal(`i_paths`),
					}, {
						Type:  `Item`,
						Name:  `http`,
						Value: Literal(`i_http`),
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
					}, {
						Type: `Item`,
						ID:   `i_http`,
						Public: Properties{
							{
								Type: `var`,
								Name: `global`,
								Value: map[string]interface{}{
									`baseUrl`: `http://localhost:11647`,
								},
							},
						},
						Functions: []Function{
							{
								Name:      `request`,
								Arguments: []string{`config`},
								Definition: `
									config = (config || {});

									// inherit certain options from global overrides
									config.contentType = (config.contentType || global.contentType);
									config.dataType = (config.dataType || global.dataType);

									var method = (config.method || 'GET').toUpperCase();
									var url = config.url;
									var xhr = new XMLHttpRequest();

									if (!url || !url.length) {
										throw "Must provide a URL to HTTP.request()";
									}

									if (global.baseUrl && url.match(/^https?:/) === null) {
										url = global.baseUrl.toString() + url;
									}

									xhr.onreadystatechange = (function (myxhr) {
										return function () {
											switch (myxhr.readyState) {
												case XMLHttpRequest.OPENED:
													if (config.log) {
														console.debug('http: ' + method + ' ' + url);
														console.debug('http: ' + JSON.stringify(config.data));
													}

													if (config.started) {
														config.started(xhr);
													}

													break
												case XMLHttpRequest.DONE:
													if (config.log) {
														console.debug('http: ' + myxhr.status.toString() + ' ' + myxhr.statusText);
													}

													if (config.done) {
														config.done({
															status: myxhr.status,
															statusText: myxhr.statusText,
															xhr: myxhr,
														});
													}

													if (myxhr.status < 400) {
														if (config.success) {
															switch (myxhr.status) {
																case 204, 205:
																	config.success(null, myxhr);
																	break;
																default:
																	if (myxhr.response.length) {
																		switch (config.dataType) {
																			case 'text':
																				config.success(myxhr.responseText, myxhr);
																				break;
																			default:
																				config.success(JSON.parse(myxhr.response), myxhr);
																				break;
																		}
																	} else {
																		config.success(null, myxhr);
																	}
															}
														}
													} else {
														var msg = myxhr.response;

														try {
															msg = myxhr.responseJSON.error;
														} catch (e) {
															;
														}

														if (config.error) {
															config.error(msg, myxhr.response, myxhr);
														}

														console.error('HTTP ' + myxhr.status.toString() + ': ' + myxhr.statusText, msg);
													}

													break
											}
										}
									})(xhr);

									xhr.open(method, url, true);

									if (config.contentType) {
										xhr.setRequestHeader('Content-Type', config.contentType);
									} else {
										xhr.setRequestHeader('Content-Type', 'application/json');
									}

									if (global.headers) {
										for (var k in global.headers) {
											xhr.setRequestHeader(k, global.headers[k]);
										}
									}

									if (config.headers) {
										for (var k in config.headers) {
											xhr.setRequestHeader(k, config.headers[k]);
										}
									}

									var body = '';

									if (config.data !== undefined) {
										switch (config.dataType) {
											case 'text':
												body = config.data.toString();
												break;
											default:
												body = JSON.stringify(config.data);
												break;
										}
									}

									xhr.send(body);
								`,
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
						Definition: `return (parent || hydra.root || Screen).width * (parseFloat(pct) / 100.0);`,
					}, {
						Name:       `vh`,
						Arguments:  []string{`pct`, `parent`},
						Definition: `return (parent || hydra.root || Screen).height * (parseFloat(pct) / 100.0);`,
					}, {
						Name:       `vmin`,
						Arguments:  []string{`pct`, `parent`},
						Definition: `return Math.min(vw(pct, parent), vh(pct, parent));`,
					}, {
						Name:       `vmax`,
						Arguments:  []string{`pct`, `parent`},
						Definition: `return Math.max(vw(pct, parent), vh(pct, parent));`,
					}, {
						Name:      `align`,
						Arguments: []string{`h`},
						Definition: `
							if (h == 'right') {
								return Text.AlignRight;
							} else if (h == 'center') {
								return Text.AlignHCenter;
							} else {
								return Text.AlignLeft;
							}`,
					}, {
						Name:      `valign`,
						Arguments: []string{`h`},
						Definition: `
							if (v == 'bottom') {
								return Text.AlignBottom;
							} else if (v == 'center') {
								return Text.AlignVCenter;
							} else {
								return Text.AlignTop;
							}`,
					},
				},
			},
		},
	}
}
