## `.site/` ##

This is the source code for the website hosted at [`umo.ci`][umoci]. The reason
for this weird structure is so that we can still have a top-level `doc/`
directory which is then included as a chapter inside the website.

The website uses [Hugo][hugo], and can be rebuilt with a simple `hugo`. To have
a development webserver use `hugo serve`. For more information, read the docs.

[umoci]: https://umo.ci/
[hugo]: https://gohugo.io/

### License ###

As with the rest of `umoci`, the website is licensed under the Apache 2.0
license. The theme [`hugo-theme-learn`][hugo-theme] is licensed under the
MIT/X11 license.

[hugo-theme]: https://github.com/matcornic/hugo-theme-learn
