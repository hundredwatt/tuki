<p align="center">
    Safely Run Commands in Productions with Tuki!
</p>
<p align="center">
    <img src="https://github.com/hundredwatt/tuki/blob/c88122fb49fdd4a3036d584be31479f011b7b390/imgs/tuki-logo.jpeg" width="200" height="200">
</p>

> **⚠️ Welcome:** This is alpha software. We are actively looking for collaborators to work on this project with us. If you are interested, please reach out or submit a pull request!


# Tuki Task Runner

Tuki provides an alternative to running commands in your production REPL or console directly. Instead, you can write your commands in a Git repository, use the standard git workflow to review and iterate on them, and then merge them into your production branch for execution.

## Features

- Execute scripts in your production environment from a Git repository.
- State is stored in the Git repository for persistence and visibility.
- Harness file for customizing how tasks are run.

## Usage

See docs [here](https://github.com/hundredwatt/tuki/tree/main/docs).

## Contributing

Contributions are welcome! Please fork the repository and submit a pull request.

## Running Tests

To run unit tests, use the following command:

```bash
go test ./...
```

To run integration tests, use the following command:

```bash
go test -tags=integration ./...
```

## License

This project is licensed under the MIT License - see the [MIT-LICENSE](./MIT-LICENSE) file for details.
