* There's lots of memory sharing happening. Apparently none of it is
  concurrency-safe. Who knew?
* BUFFER_SIZE needs to be uint8, not uint32, since it's encoded as a single
  byte.
