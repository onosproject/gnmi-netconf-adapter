include ./common.mk

##
# Add in project specific targets below
##

# Tools

.PHONY: coverage
coverage: overalls | $(GOVERALLS) ; $(info $(M) running coveralls) @ ## run coveralls (PROJECT)
	$Q $(GOVERALLS) -coverprofile=overalls.coverprofile -service=travis-ci


# this and the common clean will both executed because of ::

.PHONY: clean
clean:: ; $(info $(M) gnmi-netconf-adapter clean) @ ## clean (ADDITIONAL)
	@rm -rf  build/_output


# example of override the build target in the common makefile, you'll get a make warning about overriding
# but the return code will be ok

# .PHONY: build
# build: $(BIN) ; $(info $(M) building executable…) @ ## Build program binary (OVERRIDE)
# 	$Q CGO_ENABLED=$(CGO_ENABLED) $(GO) build \
# 		-tags release \
# 		-ldflags '-X $(MODULE)/cmd.commit=$(VERSION) -X $(MODULE)/cmd.date=$(DATE)' \
# 		-o $(BIN)/$(basename $(MODULE)) main.go

.PHONY: images
images: docker-$(PRJ_NAME) ; $(info $(M) building images...) @ ## build all docker images (ADDITIONAL)

.PHONY: images-push
images-push: images $(DOCKER_LOGIN) ; $(info $(M) pushing images...) @ ## push docker images (PROJECT)
	docker push onosproject/$(PRJ_NAME):$(PRJ_VERSION)

.PHONY: kind
kind: images ; $(info $(M) add images to kind cluster...) @ ## add images to kind (ADDITIONAL)
	@if [ "`kind get clusters`" = '' ]; then echo "no kind cluster found" && exit 1; fi
	kind load docker-image onosproject/$(PRJ_NAME):$(PRJ_VERSION)