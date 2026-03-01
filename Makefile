.PHONY: help frontend datasystem functionsystem yuanrong dashboard all

help:
	@echo "Available targets:"
	@echo "  make frontend      - Build frontend (auto-fixes go.mod path)"
	@echo "  make datasystem   - Build datasystem"
	@echo "  make functionsystem - Build functionsystem"
	@echo "  make yuanrong     - Build runtime"
	@echo "  make dashboard   - Build dashboard"
	@echo "  make all          - Build all targets and copy outputs to output/"

frontend:
	if grep -q 'yuanrong.org/kernel/runtime.*=>.*\.\./yuanrong/api/go' "frontend/go.mod"; then \
		sed -i 's|yuanrong.org/kernel/runtime.*=>.*\.\./yuanrong/api/go|yuanrong.org/kernel/runtime => ../api/go|g' "frontend/go.mod"; \
		echo "Updated frontend/go.mod: yuanrong.org/kernel/runtime => ../api/go"; \
	else \
		echo "frontend/go.mod already correct"; \
	fi
	bash frontend/build.sh
	mkdir -p output
	cp frontend/output/yr-frontend*.tar.gz output/

datasystem:
	bash datasystem/build.sh -X off -G on -i on
	mkdir -p output
	cp datasystem/output/yr-datasystem*.tar.gz output/

functionsystem:
	mkdir -p functionsystem/vendor/src
	cp datasystem/output/yr-datasystem-*.tar.gz functionsystem/vendor/src/yr-datasystem.tar.gz
	cd functionsystem/ && bash run.sh build && bash run.sh pack && cd -
	mkdir -p output
	cp functionsystem/output/yr-functionsystem*.tar.gz output/

yuanrong:
	[ -d datasystem/output/sdk ] || tar --no-same-owner -zxf datasystem/output/yr-datasystem-*.tar.gz --strip-components=1 -C datasystem/output"
	cp -ar functionsystem/output/metrics ./
	bash build.sh -l /tmp/bazelcache

dashboard:
	cd go && bash build.sh && cd -

all: frontend datasystem functionsystem yuanrong dashboard
	@echo "Build completed!"
	@echo "Copying outputs to output/..."
	mkdir -p output
	cp frontend/output/yr-frontend*.tar.gz output/
	cp datasystem/output/yr-datasystem*.tar.gz output/
	cp functionsystem/output/yr-functionsystem*.tar.gz output/
