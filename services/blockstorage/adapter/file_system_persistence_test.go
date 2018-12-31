package adapter

// TODO V1 check that we cannot open the same file twice for writing (flock?)
// TODO V1 check that we can read concurrently from different places in the file
// TODO V1 check that we don't use long locks - that concurrent reads don't wait on each other
// TODO V1 init flow - build indexes
// TODO V1 init flow - handle file corruption
// TODO V1 error during persistence
// TODO V1 tampering FS?
// TODO V1 checks and validations
// TODO V1 codec versions
// TODO V1 test that if writing a block while scanning is ongoing we will receive the new
// TODO V1 Persist block height index
// TODO V1 write test for recovering from a corrupt writing file handle
// TODO V1 write memory leak tests to make sure we close long lasting file handles
// TODO V1 file format includes a version