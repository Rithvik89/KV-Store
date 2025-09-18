package main

func processCmd(cmd string, executor *Executor) string {
	args, isValid := parseAndValidateCmd(cmd)
	if isValid {
		if args[0] == CMD_GET {
			value, ok := executor.Store.Get(args[1])
			if !ok {
				return "Key not found!"
			}
			return value
		}
		if args[0] == CMD_PUT {
			// Insert into WAL before inserting into MemStore
			entry := &WALEntry{
				Op:    "PUT",
				Key:   args[1],
				Value: args[2],
			}
			if !executor.WAL.writeToWAL(entry) {
				return "Failed to write to WAL!"
			}
			// Now insert into MemStore
			executor.Store.Put(args[1], args[2])
			return "Successfully inserted! for key: " + args[1]
		}
		if args[0] == CMD_DELETE {
			// Insert into WAL before deleting from MemStore
			entry := &WALEntry{
				Op:    "DELETE",
				Key:   args[1],
				Value: "",
			}
			if !executor.WAL.writeToWAL(entry) {
				return "Failed to write to WAL!"
			}
			// Now delete from MemStore
			ok := executor.Store.Delete(args[1])
			if !ok {
				return "Key not found!"
			}
			return "Successfully deleted! for key: " + args[1]
		}
	}

	return "Invalid input format!"
}
