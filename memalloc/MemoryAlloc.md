## Simple Memory Allocator

### Memory Layout of a Program

Split into 5 different sections

1. Text Section
2. Data Section (non-zero initialized static data)
3. Block Started by Symbol (BSS) (zeo initialized static data)
4. Heap
5. Stack

The heap and stack grow in **opposite** directions. The `data, bss and heap` sections can be referred to as the _Data Segment_

`Text Section` stores the **binary** instructions to be _executed_ by the processor.

If we want to `allocate` memory to the **Heap** we have to increment the `brk` pointer. This points to the _end_ of the heap.
Similarly, to `release` memory we would have to _decrement_ the `brk` pointer.

### Sbrk() in Linux

---

`sbrk(0)` gives us the current address of the program brk
`sbrk(x)` with a positive value `increments` brk by x bytes. As a result `allocating` memory
`sbrk(-x)` with a negative value `decrements` the pointer by x bytes.

When you use sbrk(0) you get the current "break" address.
When you use sbrk(size) you get the previous "break" address, i.e. the one before the change.

### Freeing Memory

---

Allocating the memory is fairly simple if we only call `sbrk()` or `mmap()`. The tricky part is freeing the memory. We have to know the `size` of the memory block to be freed.

To do this we have to store the size information of the allocated blcok somewhere.

`Heap Memory` provided by the OS is **contiguous**. We can only release memory that is at the END of the heap not from the middle.

Freeing memory for now will mean that it is _free_ to be used later on a different `malloc()` call.

We also have to store whether a block is free or not free somwhere.

### Header

---

By using a `header struct` we can store this information. When a program requests new memory we calculcate `total_size = header_size + size` and pass this `total_size` to the `sbrk()` call.

Now the memory blocks will look like:

```
------  ---------------------
header | actual memory block |
------- ---------------------
```

We cant be sure that the blocks allocated by our `malloc` are _contiguous_ there could be other calls that added memory in between our blocks.

Due to this we have to have a way to `traverse` through our blocks for memory. To keep track of the memory allocated by our `malloc` is to keep it in a **linked list**.

Each header will have a `next` pointer that points to the next allocated block of memory.

#### Union

---

Using a `union` makes the header end up on a memory `address` aligned to 16 bytes. The union guarantees that the _end_ of the header is memory aligned. The end is where the actual memory block begins, therefore the memory provided to the caller will be aligned to 16 bytes.

#### Casting Pointers to different Types

---

Example: `(header_t*) block`

This is casting the block pointer which is currently a pointer of type `void*` to the type `header_t`.
When we do this it tells the compiler: "Treat block as a pointer to a header_t structure."

**Pointer arithmetic** depends on the _type_ of pointer. When we do `(header_t*)block-1` we are moving back by the size of
ONE `header_t` structure.

#### Things I learned

---

- When allocating memory using these functions we are directly interacting with the memory _heap_.
- `Pointer Arithmetic` I was unaware you can move pointers by the size of a struct (i.e `(header_t*) block)`)
- When doing `sbrk(0 - sizeof(header_t) - header->s.size);` adding the zero infront of the subtraction is a stylistic way of writing a negative offset. It is equivalent to doing `sbrk(-(sizeof(header_t) + header->s.size));`
