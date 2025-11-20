

FILES=lsf_exporter lshosts Solver-Standard.csv \
			lsf_exporter_test.sh \
		 	./custom/config/lsf_exporter.env \
	    ./custom/config/lsf_exporter.service

build: lsf_exporter

debug: lsf_exporter lshosts
	rsync -ia $? collector-it-aws01:Tools

deploy: $(FILES)
	rsync -ia $? collector-it-aws02:Tools
	rsync -ia $? collector-fr-aws02:Tools
	echo "Now run stop/start for the systemd services"
	echo "sudo cp lshosts /usr/local/bin"
	echo "sudo systemctl stop lsf_exporter.service && sudo cp lsf_exporter /usr/local/bin && sudo systemctl start lsf_exporter && systemctl status lsf_exporter.service"

lsf_exporter: lsf_exporter.go collector/*go
	go build -o lsf_exporter

