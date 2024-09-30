# pbclient

A simple, non-feature complete Go client library for interacting with a Pocketbase instance.

## Overview

The pbclient package provides a Go API to interact with Pocketbase, allowing operations
such as authentication, fetching logs, creating records, fetching records by ID,
filtering records, and updating records.

The package utilizes the `requests` package from [github.com/blakepatteson/gorequests](https://github.com/blakepatteson/gorequests)
(a simple http wrapper) for making HTTP requests."

## Features

- Authenticating as admin or user.
- Fetching logs.
- Creating new records in a specified collection.
- Getting records from a collection with optional parameters like filter and expand.
- Getting records by ID.
- Geting all records from a collection.
- Updating an existing record.
- Filtering and expanding results for more complex queries

## Initialization

```go
pb, err := NewPocketbase("http://your-pocketbase-url.com", "your-username", "your-password", true) // true for admin, false for user
```

## Creating a new record

```go
out, err := pb.CreateRecord("collectionName", `your-record-data-in-json-format`)
```

## Fetching all logs

```go
allLogs, err := pb.GetAllLogs()
```

## Fetching a record by ID

```go
record, err := pb.GetRecordById("collectionName", "recordId")
```

## Fetching records based on a filter

```go
filteredRecords, err := pb.GetFilteredRecords("collectionName", "your-filter-string")
```

## Updating a record

```go
updatedId, err := pb.UpdateRecord("collectionName",
                        `{"someKey":"someValue","anotherField":"anotherValue"}`, "recordId")
```

## Deleting a record

```go
statusCode, err := pb.DeleteRecord("collectionName", "0f8l6f7zm98vhle")
```

## Dependencies

[github.com/blakepatteson/gorequests](https://github.com/blakepatteson/gorequests)

## Testing

To run tests for the pbclient package, you'll need to have a local instance of PocketBase
running. You can easily set this up using the provided Dockerfile in the repository.

## Setting up PocketBase locally

Ensure you have Docker installed on your machine.

Navigate to the root directory of the repository and build the Docker image using the
provided Dockerfile:

```bash
sh buildLocalPb.sh
```

or just

```
docker build -t pocketbase-local .
docker run -p 8080:8080 pocketbase-local
```

This script will use the Dockerfile to create a local PocketBase image and start a
container that exposes the service on port 8080.

Once the container is running, the PocketBase service will be accessible at http://0.0.0.0:8080.

## Running Tests

With the PocketBase service running locally, navigate to the pbclient directory and execute your tests:

```bash
go test -v
```

Make sure the configuration in your tests (like the PocketBase URL) matches the local setup (http://0.0.0.0:8080).

## Contributing

Feel free to contribute to this project by opening issues or submitting pull requests,
especially if adding features for more functionality.

## License

This project is licensed under the [MIT license](/LICENSE).
