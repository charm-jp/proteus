regenerate:
	- make -C $(GOPATH)/src/github.com/charm-jp/proteus reinstall
	- proteus -p github.com/charm-jp/proteus/example -p github.com/charm-jp/proteus/example/categories -f $(GOPATH)/src/github.com/charm-jp/proteus/example/protos --verbose
	# Need to generate twice to avoid bug where rpc would detect the
	# autogenerated methods as real ones, not generating them if proteus was
	# called again.
	- proteus -p github.com/charm-jp/proteus/example -p github.com/charm-jp/proteus/example/categories -f $(GOPATH)/src/github.com/charm-jp/proteus/example/protos --verbose
