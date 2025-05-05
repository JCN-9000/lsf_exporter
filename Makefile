

debug: lsf_exporter
	scp -p lsf_exporter collector-it-aws01:

deploy: lsf_exporter
	ssh -n ec2-user@collector-fr-aws01 sudo -n systemctl stop  lsf_exporter.service
	scp -p lsf_exporter collector-fr-aws01:/opt/exporters
	ssh -n ec2-user@collector-fr-aws01 sudo -n systemctl start lsf_exporter.service

	ssh -n ec2-user@collector-it-aws01 sudo -n systemctl stop  lsf_exporter.service
	scp -p lsf_exporter collector-it-aws01:/opt/exporters
	ssh -n ec2-user@collector-it-aws01 sudo -n systemctl start lsf_exporter.service

	ssh -n ec2-user@collector-fr-aws02 sudo -n systemctl stop  lsf_exporter.service
	scp -p lsf_exporter collector-fr-aws02:/usr/local/bin/lsf_exporter
	ssh -n ec2-user@collector-fr-aws02 sudo -n systemctl start lsf_exporter.service

	ssh -n ec2-user@collector-it-aws02 sudo -n systemctl stop  lsf_exporter.service
	scp -p lsf_exporter collector-it-aws02:/usr/local/bin/lsf_exporter
	ssh -n ec2-user@collector-it-aws02 sudo -n systemctl start lsf_exporter.service

lsf_exporter: lsf_exporter.go collector/*go
	go build

