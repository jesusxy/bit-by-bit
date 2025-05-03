## 📚 Stack Direction: Up or Down?

Write a program in C that can compute if the stack grows "up or down"

This is a small C program designed to determine the direction of stack growth on the system, whether the stack grows upwards (toward higher memory addresses) or downwards (toward lower addresses).

### 🥅 Goal
- Deepen understanding of how the stack operates at runtime
  - Specifically: In which direction does the stack grow?
- Practice writing minimal, diagnostic C programs to explore system-level behavior

### 🔬 Alternative Approach: Cross-Frame Comparison
```c
bool upordown(int *other) {
    int x;
    
    if (!other) {
        return upordown(&x);
    } else {
        return &x > other;
    }
}
```

This method compares the address of a local variable across two recursive stack frames:
1. First call passes a null pointer and captures the address of x.
2. Second call compares a new x's address to the one from the first frame.
3. The relative position of the two frames reveals stack direction.

### 🧠 What I Learned
- Stack frames are created each time a function is called. In this example, the address of the variable x changes between recursive calls.
- The comparison `&x > other` effectively checks whether the **second** frame's local variable (x) has a higher address than the one from the _first_ frame.
- This technique works because the first call passes a `null` pointer, triggering a second call that compares the addresses of local variables in two different frames.
