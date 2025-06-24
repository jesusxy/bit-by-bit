## SQLite Clone - C

### Part I

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

---

#### Freeing Memory

In our `close_input_buffer()` we call free twice. We have to explicitely free both the `buffer` and the `InputBuffer` struct.
They are two separate allocations of memory. The `buffer` field is a **pointer** to memory that is dynamically allocated. This is in a different location and address than the `InputBuffer` struct.

If we dont explicitly free the memory pointed by `input_buffer->buffer` it will result in a _memory leak_.

Calling `free(input_buffer)` only frees the memory allocated for InputBuffer struct but it wont traverse or deallocate memory that the
structs fields point to. The `free` function only operates at a single memory allocation level.

For composite structures we want to

1. free nested allocations first
2. free the struct itself

---

### Part II

Objective of this part is to create a compiler that parses the input string and outputs `bytecode`. This will then get passed to our VM to be processed.

- C does not support exceptions which is why we use `enum` result codes
- The compiler will complain if a switch statement does not handle a member of the enum, giving us confidence that all results of a function were handled

---

### Part III

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

Strings in C are `null terminated character sequences` \
Example: "Alice"
It will be stored as: `['A', 'l', 'i', 'c', 'e', '\0', ...]`

"alice@example.com" would be stored as `['a', 'l', 'i', 'c', 'e', '@', 'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', '\0', ...]`

---

### Part V

Peristing records to memory will be done by the **Pager**.

- We ask the pager for page number x
- it first looks in the cache
- if there is a cache miss, it copies data from disk into memory

Each `PAGE` will hold a **fixed number** of `rows` **(ROWS_PER_PAGE)**. The database file will be divided into pages of size **(PAGE_SIZE)**, these are fixed chunks of memory (4096 bytes).

Pages act as units of `reading and writing` data to disk. Reading data in "chunks" minimizes the number of disk operations, a single page read will handle multiple rows.
To locate a specific row:

- jump to a specific page
- locate the row within that page

Example: Database file = 16kb, Page Size = 4 kb. The file will be divided into 4 pages

```
Page 0: 0 - 4095
Page 1: 4096 - 8191
Page 2: 8192 - 12287
Page 3: 12288 - 16383

```

### Loading DB

When we load the database file we need to know how many **rows** are already present in the file. To do this we have to `divide` the total file size **(file_length)** by the
size of each row **(ROW_SIZE)**

Example:

```
file_length = 1168 bytes
ROW_SIZE = 292 bytes

num_rows = 1168 / 292 = 4 rows of records
```

Lets take another example:

```
file_length: 8192 bytes (file size)
PAGE_SIZE = 4096 bytes
ROW_SIZE = 292 bytes

num_pages = file_length / PAGE_SIZE = 8192 / 4096 = 2 full pages
ROWS_PER_PAGE = PAGE_SIZE / ROW_SIZE = 4096 / 292 = 14 rows per page
num_rows = file_length / ROW_SIZE = 8192 / 292 = 28 rows total in file
```

With the numbers above we have a database with 28 rows, these rows are spread across
2 pages (14 rows per page)

| **Page #** | **Rows** | **Offset Index** | **Range of Bytes** |
| ---------- | -------- | ---------------- | ------------------ |
| 0          | 0 to 13  | 0 to 13          | 0 to 4095          |
| 1          | 14 to 27 | 0 to 13          | 4096 to 8191       |
| 2          | 28 to 41 | 0 to 13          | 8192 to 12287      |
| 3          | 42 to 55 | 0 to 13          | 12288 to 16383     |
| 4          | 56 to 69 | 0 to 13          | 16384 to 20479     |

Each page can hold exactly `ROWS_PER_PAGE` amount of rows, the offset always starts at 0 and goes up to `ROWS_PER_PAGE - 1`.

The offset for each row is essentially an "index" we use to locate the row within the page.

**Example:** `row_position = row_offset x ROW_SIZE = 2 x 32 = 64 bytes`

---

### Part VII

**B-Trees** are used to store `Indexes` in SQLite.
**B+ Trees** are used to store `Tables` in SQLite.

Our current table format where we store only rows (no metadata) is space efficient.
Insertion is fast because we append to the **end** of the table.
Finding a particular row is time consuming because we have to scan the **entire** table.

If we stored the table as an array and kept it sorted by _id_ we could perform `Binary Search` to find
and id. Insertion would be slow though because we would have move and reorganize rows to make space.

| Operation     | Unsorted Array | Sorted Array | Tree of Nodes |
| ------------- | -------------- | ------------ | ------------- |
| Insertion     | O(1)           | O(n)         | O(log(n))     |
| Deletion      | O(n)           | O(n)         | O(log(n))     |
| Lookup by Key | O(n)           | O(log(n))    | O(log(n))     |

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
