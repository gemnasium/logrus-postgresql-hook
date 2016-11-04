FROM postgres
ADD migrations/* /docker-entrypoint-initdb.d/
