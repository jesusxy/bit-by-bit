## 🧱 Simple Memory Allocator in C

A minimalist implementation of malloc and free, written in C using the sbrk system call. This project explores how memory management works under the hood by building a custom allocator from scratch.

### Program Memory Leayout (Overview)

Modern programs in memory are divided into 5 sections

1. Text Segment - stores binary instructions to be executed by the CPU
2. Data Section - holds initialized global / static variables
3. Block Started by Symbol (BSS) - holds uninitialized (zeroed) global/static variables
4. Heap - grows **upward**, used for dynamic allocation(`malloc`)
5. Stack - grows **downward**, used for function calls and local variables

The heap and stack grow in **opposite** directions. The `text, data, bss and heap` are often grouped as the _Data Segment_

`Text Section` stores the **binary** instructions to be _executed_ by the processor.

If we want to `allocate` memory to the **Heap** we have to increment the `brk` pointer. This points to the _end_ of the heap.
Similarly, to `release` memory we would have to _decrement_ the `brk` pointer.

---

### 🛠 How `sbrk` works
To dynamically allocate memory on the heap, we use the `sbrk()` system call.
- `sbrk(0)` returns the current program break (end of heap)
- `sbrk(x)` **increments** break by `x` bytes, `allocating` memory
- `sbrk(-x)` **decrements** break by `x` bytes, `releasing` memory

Note: When you use sbrk(0) you get the current "break" address. When you use sbrk(size) you get the previous "break" address (before incrementing), i.e. the one before the change. Memory allocation is **easy**; memory freeing is harder.

---

### ♻️ Freeing Memory

Freeing memory is a bit trickier. We have to know the `size` of the memory block to be freed.

To `free` memory, we must:
- Know the **size** of the block to release
- Track which blocks are `free`
- Optionally **reuse** freed blocks in future `malloc` calls

Since heap memory from the OS is _contiguous_, we can only return memory to the OS if its at the **END** of the heap. Otherwise, freeing just marks a block as _reusable_

---

### ⛓️‍💥 Design: Block Header and Linked List

Each allocated block begins with a header struct:
```c
struct header {
    size_t size;
    int free;
    struct header* next;
};
```
- This metadata helps us find and reuse memory blocks
- We align each block to 16 bytes for safety and performance using a `union`
- Blocks are stored in a **linked list** for traversal and reallocation

By using a `header struct` we can store this information. When a program requests new memory we calculcate `total_size = header_size + size` and pass this `total_size` to the `sbrk()` call.

Now the memory blocks will look like:

```
------  ---------------------
header | actual memory block |
------- ---------------------
```
> 📌 We cannot assume blocks are contiguous due to other memory allocations.
We cant be sure that the blocks allocated by our `malloc` are _contiguous_ there could be other calls that added memory in between our blocks.

#### Union

Using a `union` makes the header end up on a memory `address` aligned to 16 bytes. The union guarantees that the _end_ of the header is memory aligned. The end is where the actual memory block begins, therefore the memory provided to the caller will be aligned to 16 bytes.

---

### 🔄 Pointer Casting & Arithmetic

```c
(header_t*) block
```
This cast tells the `compiler` to interpret the **block** pointer as pointing to a `header_t` struct.

Pointer arithmetic follows the **type size**, so:
```c
(header_t*)block - 1
```
moves backward by _ONE_ `header_t` unit:

Example:
```c
sbrk(-(sizeof(header_t) + header->size));
```
This frees a block by subtracting the total size from the heap break.

---

### 📚 What I Learned

- When allocating memory using these functions we are directly interacting with the memory _heap_.
- `Pointer Arithmetic` I was unaware you can move pointers by the size of a struct (i.e `(header_t*) block)`)
- When doing `sbrk(0 - sizeof(header_t) - header->s.size);` adding the zero infront of the subtraction is a stylistic way of writing a negative offset. It is equivalent to doing `sbrk(-(sizeof(header_t) + header->s.size));`
