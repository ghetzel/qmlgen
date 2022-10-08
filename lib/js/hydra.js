'use strict';

window.hydra = {
    types: {
        stringify: function(value) {
            switch (typeof value) {
            case 'function':
                return value();
            case 'object':
                return JSON.stringify(value);
            default:
                return value.toString();
            }
        },
        resolve: function(value) {
            switch (typeof value) {
            case 'function':
                return value(value);
            default:
                return value;
            }
        },
    },
    dom: {
        rxSplitSelector: (/^([\w\.\-\_]+)(\[[^\]]+\])?(\.[\w\.]+)?$/),
        parse: function(selector) {
            var parts = selector.toString().split(window.hydra.dom.rxSplitSelector);

            if (parts.length == 5) {

                var tagClassPre = parts[1].split('.');
                var attrset = parts[2];
                var tagClassPost = parts[3];

                if (tagClassPost) {
                    tagClassPost = tagClassPost.split('.');
                } else {
                    tagClassPost = [];
                }

                var tagName = tagClassPre[0];
                var classes = tagClassPre.slice(1).concat(tagClassPost);
                var attrs = {};

                if (attrset) {
                    var attrset = attrset.split(/[\,\;]+/);

                    for(var pair in attrset) {
                        var p = pair.split('=', 2);

                        if (p.length == 2) {
                            attrs[p[0]] = p[1];
                        }
                    }
                }

                return {
                    'tag':        tagName,
                    'classes':    classes,
                    'attributes': attrs,
                };
            } else {
                throw 'cannot parse into DOM elements: invalid selector string "' + selector + '"';
            }
        },
        create: function(tagName, content, attributes){
            var extracted = window.hydra.dom.parse(tagName);

            tagName = extracted.tag;
            attributes = (attributes || {});

            for(var k in extracted.attributes) {
                attributes[k] = extracted.attributes[k];
            }

            if (extracted.classes.length) {
                var cls = (attributes['class'] || '').split(/\s+/g);

                cls = cls.concat(extracted.classes);
                cls = cls.join(' ');

                attributes['class'] = cls.trim();
            }

            var el = document.createElement(tagName);

            for(var i in attributes) {
                var v = attributes[i];

                el.setAttribute(i, hydra.types.stringify(attributes[i]));
            }

            // get at the real nutmeat of the underlying value
            content = window.hydra.types.resolve(content);

            switch (typeof content) {
            case 'undefined':
                break;
            case 'object':
                var children = [];

                for(var i in content) {
                    var sub = window.hydra.types.resolve(content[i]);

                    if (typeof sub !== 'object') {
                        throw 'subelement must be an object';
                    } else if (!sub.content) {
                        throw 'subelement must have "content" property';
                    } else if (Number.isInteger(i) && !sub.tag) {
                        throw 'subelement must have "tag" property';
                    }

                    el.appendChild(window.hydra.dom.create(sub.tag, sub.content, sub.attributes));
                }

                break;
            default:
                el.appendChild(document.createTextNode(hydra.types.stringify(content)));
            }

            // document.body.insertBefore(newDiv, currentDiv);
            return el;
        },
    },
    http: {
        defaults: {
            url: '',
            parser: function(xhr) {
                try {
                    return JSON.parse(xhr.responseText);
                } catch(e) {
                    switch(xhr.responseType) {
                    case 'text':
                        return xhr.responseText;
                    case 'document':
                        return xhr.responseXML;
                    default:
                        return xhr.response;
                    }
                }
            },
        },
        request: function(options) {
            options = (options || {});

            switch(options.method.toString().toLowerCase()) {
            case 'get':
            case 'post':
            case 'put':
            case 'delete':
            case 'options':
            case 'patch':
                break;
            default:
                throw new Error('Unsupported HTTP method "'+options.method.toString()+'"');
            }

            if (options.url) {
                options.url = (window.hydra.http.defaults.url || '') + options.url.toString();
            } else {
                throw new Error('HTTP request must specify a URL or path to be joined with window.hydra.http.defaults.url');
            }

            // Set up our HTTP request
            var xhr = new XMLHttpRequest();

            // populate headers from global defaults
            for(var header in (window.hydra.http.defaults.headers || {})) {
                xhr.setRequestHeader()(header, window.hydra.http.defaults.headers[header]);
            }

            // populate headers from options object
            for(var header in (options.headers || {})) {
                xhr.setRequestHeader()(header, options.headers[header]);
            }

            // Setup our listener to process request state changes
            xhr.onreadystatechange = function() {
                // Only run if the request is complete
                switch (xhr.readyState) {
                case 0:  // UNSENT
                    return;
                case 1:  // OPENED
                    console.debug('http:', options.method.toUpperCase(), options.url, options);
                    return;
                case 2:  // HEADERS_RECEIVED
                    return;
                case 3:  // LOADING
                    return;
                case 4:  // DONE
                    break;
                }

                // parse response body
                var targetElement = undefined;
                var parser = (options.parser || window.hydra.http.defaults.parser);

                switch (typeof options.target) {
                case 'string':
                    targetElement = document.querySelector(options.target);
                    break;
                default:
                    targetElement = options.target;
                    break;
                }

                var defaultParsed = window.hydra.http.defaults.parser(xhr, null);
                var parsed = parser(xhr, defaultParsed);

                // call success/error paths, populate target element if specified
                if (xhr.status >= 200 && xhr.status < 300) {
                    if (typeof options.success === 'function') {
                        success(parsed, xhr);
                    }

                    switch (typeof parsed) {
                    case 'object':
                        targetElement.innerHTML = parsed.outerHTML;
                        break;
                    default:
                        targetElement.innerHTML = parsed;
                    }
                } else {
                    if (typeof options.error === 'function') {
                        error(parsed, xhr);
                    }

                    if (targetElement) {
                        targetElement.innerHTML = ('ERROR: HTTP ' + xhr.statusText + ': ' + parsed.toString());
                    }
                }

                if (typeof options.done === 'function') {
                    done(parsed, xhr);
                }
            };

            xhr.open(options.method.toUpperCase(), options.url);
            xhr.send();
        },
    },
    message: function(id, data) {
        console.log(id, data);
    },
    quit: function() {
        return window.hydra.request({
            method: 'delete',
            url:    '/hydra',
        });
    },
};