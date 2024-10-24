# Touch file

Touch files are used to coordinate between multiple processes, analogously to
the use of `sync.Mutex`.

The problem with touch files is that they are not _really_ atomic and can lead
to races. This package provides a multi-process-safe way to use touch files.
