# How to deploy?

## Prerequisites
- Install Docker in order to run the steps provided in the next section


## Hello, World!


Fenix is an applicatie that extracts data from a (healthcare-)system and converts it to FHIR-compliant-data. For validation, it uses a HAPI-FHIR-server that uses FHIR-profiles specified in https://build.fhir.org/ig/SanteonNL/sim-on-fhir/ (source in https://github.com/SanteonNL/sim-on-fhir). 

Here we will explain how to start both Fenix and HAPI server using a Docker compose file. 

To get started, create a new project directory, e.g.:

```
md fenix
cd fenix
```

Copy the following YAML file and save it as docker-compose.yaml in the new  directory.

```
services:
  fenix:
    image: crfhir.azurecr.io/fenix:latest
    ports:
      - "1357:1357"
    volumes:
      - "./data/fenix:/opt/fenix/data"
      - "./ssl/fenix:/opt/fenix/ssl"
  hapi:
    image: hapiproject/hapi:latest
    environment:
      spring.config.location=/opt/hapi/temp.application.yaml
    #  hapi.fhir.fhir_version: R4
    #  hapi.fhir.partitioning.allow_references_across_partitions: "false"
    #  hapi.fhir.implementationguides:
    ###    example from registry (packages.fhir.org)
    #  swiss:
    #    name: swiss.mednet.fhir
    #    version: 0.8.0
    #    reloadExisting: false
    #    installMode: STORE_AND_INSTALL
    #      example not from registry
    #      ips_1_0_0:
    #        packageUrl: https://build.fhir.org/ig/HL7/fhir-ips/package.tgz
    #        name: hl7.fhir.uv.ips
    #        version: 1.0.0
    #    supported_resource_types:
    #      - Patient
    #      - Observation
    ports:
      - "127.0.0.1:4004:8080"
    volumes:
      - "./config/hapi/temp.application.yaml:/opt/hapi/temp.application.yaml"
      - "./data/hapi:/usr/local/tomcat/target"
```

```
docker compose pull
docker compose up
```
After the services have started you can try the following endpoints:

- Fenix status page: http://localhost:1357/status


<!-- TODO: You could also start these applications as seperate containers, e.g. HAPI: docker run -p 8090:8080 -v %userprofile%/Repositories/fenix/hapi/config:/configs -e "--spring.config.location=file:///configs/temp.application.yaml" hapiproject/hapi:latest -->
