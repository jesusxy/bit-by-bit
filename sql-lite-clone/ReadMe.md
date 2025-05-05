## 🗃️ tiny-sql

> A toy SQL database engine built from scratch in C.

This codebase is a direct implementation of [cstack's "Let's Build a Simple Database" tutorial.](https://cstack.github.io/db_tutorial)
The goal is not originality, but deep understanding. I’m using this project to explore how relational databases work at a low level, how rows are serialized, pages are managed, and queries are executed.

---

### 🥅 Goals

Understand how SQL statements are parsed, stored, and executed

- Explore how databases use B-Trees, paging, and cursors
- Implement a custom REPL for running basic SQL-like commands
- Learn low-level file I/O, memory layouts, and serialization

---

### 🚀 Getting Started

```shell
gcc main.c -o tiny_sql
./tiny_sql dbfile
```

You'll see a prompt:

```c
db > insert 1 "alice" "alice@example.com"
Executed.
db > select
(1, alice, alice@example.com)
Executed.
```

---

### 📚 Learnings

Some of the core ideas this project explores:

- How to build a persistent row format with byte-level control
- Why databases use fixed-size pages for performance
- How B-Trees help with efficient data lookup
- What a cursor is and how it's used for scanning records
- Why serialization and deserialization are fundamental in storage engines

---

### 🙏 Credits

This project follows and expands on [cstack's database tutorial](https://cstack.github.io/db_tutorial). Credit to @cstack for the inspiration and pedagogical clarity.
