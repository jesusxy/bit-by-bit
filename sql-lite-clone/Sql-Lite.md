## SQLite Clone - C

---

`SQL Query` goes through the following components.
On the **front-end**

- tokenizer
- parser
- code generator

The output of this query will be `sqlite virtual machine bytecode` (a compiled program that can operate on the db)

The **backend** consists of the following components:

- virtual machine
- b tree
- pager
- os interface

#### Freeing Memory

---

In our `close_input_buffer()` we call free twice. We have to explicitely free both the `buffer` and the `InputBuffer` struct.
They are two separate allocations of memory. The `buffer` field is a **pointer** to memory that is dynamically allocated. This is in a different location and address than the `InputBuffer` struct.

If we dont explicitly free the memory pointed by `input_buffer->buffer` it will result in a _memory leak_.

Calling `free(input_buffer)` only frees the memory allocated for InputBuffer struct but it wont traverse or deallocate memory that the
structs fields point to. The `free` function only operates at a single memory allocation level.

For composite structures we want to

1. free nested allocations first
2. free the struct itself
