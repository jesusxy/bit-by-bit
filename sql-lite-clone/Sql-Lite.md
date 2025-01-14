## SQLite Clone - C

---

### Part I

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

### Part II

---

Objective of this part is to create a compiler that parses the input string and outputs `bytecode`. This will then get passed to our VM to be processed.

- C does not support exceptions which is why we use `enum` result codes
- The compiler will complain if a switch statement does not handle a member of the enum, giving us confidence that all results of a function were handled

### Part III

---

For now, this db will:

- support two operations: inserting a row and printing all rows
- reside only in memory (no persistence to disk)
- support a single, hard-coded table

The db schema will be:

| Column   | Type         |
| -------- | ------------ |
| id       | integer      |
| username | varchar(32)  |
| email    | varchar(255) |

The **insert** statement will be `insert 1 cstack foo@bar.com`

To copy the data given by the insert command, we need to create a data structure that represents the table.

The plan is:

- Store rows in **blocks** of memory called `pages`
- Each page stores as many rows as it can fit
- Rows are **serialized** into compact representations with each page
- Pages are only allocated as needed
- Keep a fixed-size array of **pointers** to pages

The serialized row will now look like:
| Column | Size (bytes) | Offset |
|-----------|--------------|--------|
| id | 4 | 0 |
| username | 32 | 4 |
| email | 255 | 36 |
| total | 291 | |

We set the size of page to `4kb` because this is the size of a page in most virtual memory systems. One db page corresponds to one page used by the OS.

The OS will move pages in and out of memory as **whole** units instead of breaking them up.

#### Takeaways | TIL

---

1. (Struct\*)0 is a type cast telling the compiler to treat 0 as a pointer to a structure of type Struct. Zero is used because its the `null pointer constant`, it doesnt point to any memory.
2. The entire expression `sizeof(((Struct*)0)->Attribute)` calculates the size of member **Attribute** in the **Struct** type

The purpose of `(Struct*)0` is to avoid the need for an acutal **instance** of the structure. Its useful when we need to compute **sizes** of structure members _dynamically_ at compile time.

`sizeof` operates on **types** or **expressions** to compute `sizeof(id)` within `Row` we need a **context** where the compiler knows we are referring to `id` as part of the struct.

When using the `->` operator it requires a _pointer_ to access the members of the struct. Since `Row` is a **type** and not an **instance** doing `(Struct*)0` gives the compiler enough context to "pretent" there is a structure instance at address 0. It allows the compiler to resolve the members type without memory access.

3. 'Initializer element is not a compile time constant' error

When compiling the program after adding the page / row functionality I was receiving the above error. To fix this issue I reordered the file and moved _enums_ and _structs_ atop the file before any constants.

What I learned is that the compiler processes the file from top to bottom. The issue the compiler was complaining about was due to not having these definitions first before their use.

Constants like `EMAIL_SIZE` depend on the size of `Row` members. If Row is not declared, the compiler cannot resolve the dependency.

The best practice is to declare `structs, enums, typedef` at the beginning of the file or in a header file.
