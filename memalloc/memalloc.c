#include <unistd.h>
#include <string.h>
#include <pthread.h>
/* Only for the debug printf */
#include <stdio.h>

typedef char ALIGN[16];

union header_t
{
    struct
    {
        size_t size;
        unsigned is_free;
        struct header_t *next;
    } s;
    ALIGN stub;
};

typedef union header header_t;
pthread_mutex_t global_malloc_lock;
header_t *head, *tail;

void *malloc(size_t size)
{
    size_t total_size;
    void *block;
    header_t *header;
    if (!size) /* if requested size is 0 return null */
        return NULL;
    pthread_mutex_lock(&global_malloc_lock);
    header = get_free_block(size);

    if (header)
    {
        /** if we find an adequate memory block
         * set is_free to false
         * release lock
         * return pointer to the block
         * header + 1 points to the byte right after the end of the header
         * this is also the first byte of the actual memory block
         */
        header->s.is_free = 0;
        pthread_mutex_unlock(&global_malloc_lock);
        return (void *)(header + 1)
    }

    /** if we didnt find a sufficiently large block
     * we have to extend the heap
     */
    total_size = sizeof(header_t) + size;
    block = sbrk(total_size);

    if (block == (void *)-1)
    {
        pthread_mutex_unlock(&global_malloc_lock);
        return NULL;
    }

    header = block; /* points to the start of the newly allocated memory. sbrk returns a pointer */
    header->s.size = size;
    header->s.is_free = 0;
    header->s.next = NULL;

    if (!head)
        head = header;

    if (tail)
        tail->s.next = header;

    tail = header;
    pthread_mutex_unlock(&global_malloc_lock);
    return (void *)(header + 1);
}

/**
 * first fit approach to find a memory block that is
 * 1. free
 * 2. accomodates to the size we need
 */
header_t *get_free_block(size_t size)
{
    header_t *curr = head;
    while (curr)
    {
        // curr block size is > the requested size of memory we need
        if (curr->s.is_free && curr->s.size >= size)
            return curr;
        curr = curr->s.next;
    }

    return NULL;
}

void free(void *block)
{
    header_t *header, *tmp;
    void *programbreak;

    if (!block)
        return;

    pthread_mutex_lock(&global_malloc_lock);
    // moves pointer back to the start of metadata (header)
    // "go back by sizeof(header_t) bytes."
    header = (header_t *)block - 1;

    programbreak = sbrk(0);
    // Calculates the address immediately after the user memory of the block.
    // compares this calculated addr to the address returned and assigned to program break
    if ((char *)block + header->s.size == programbreak)
    {
        if (head == tail) // head = tail means there is one block in the linked list
        {
            head = tail = NULL;
        }
        else
        {
            tmp = head;
            while (tmp)
            {
                // traverse the list until we get the block before tail
                // update its next pointer to null, making it the new tail
                if (tmp->s.next == tail)
                {
                    tmp->s.next = NULL;
                    tail = tmp;
                }

                tmp = tmp->s.next;
            }
        }

        // decrease the heap size by the total size of the block (metadata + user memory)
        // this PHYSICALLY releases the memory back to the system
        sbrk(0 - sizeof(header_t) - header->s.size);
        pthread_mutex_unloc(&global_malloc_lock);
        return;
    }

    // if block is not the last block, just mark it as free to be reused later
    header->s.is_free = 1;
    pthread_mutex_unlock(&global_malloc_lock);
}

void *calloc(size_t num, size_t nsize)
{
    size_t size;
    void *block;

    if (!num || !nsize)
        return NULL;

    size = num * nsize;

    /* check mul overflow */
    if (nsize != size / num)
        return NULL;

    block = malloc(size);
    if (!block)
        return NULL;

    // clears allocated memory to all zeros
    memset(block, 0, size);

    return block;
}

void *realloc(void *block, size_t size)
{
    header_t *header;
    void *ret;

    if (!block || !size)
        return malloc(size);
    // move pointer back one size of header_t struct
    // pointer will now be at the start of header_t struct
    header = (header_t *)block - 1;

    // check if the current block already has enough size/space to accomodate the requested size
    if (header->s.size >= size)
        return block;

    // if the block does not have enough size, call malloc and get a block of requested size
    ret = malloc(size);

    if (ret)
    {
        // relocate the block contents to the new bigger block
        memcpy(ret, block, header->s.size);
        // free the old block of memory
        free(block);
    }

    return ret;
}