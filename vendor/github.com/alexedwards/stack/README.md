# Stack <br> [![Build Status](https://travis-ci.org/alexedwards/stack.svg?branch=master)](https://travis-ci.org/alexedwards/stack)  [![GoDoc](http://godoc.org/github.com/alexedwards/stack?status.png)](http://godoc.org/github.com/alexedwards/stack)

Stack provides an easy way to chain your HTTP middleware and handlers together and to pass request-scoped context between them. It's essentially a context-aware version of [Alice](https://github.com/justinas/alice).

[Skip to the example &rsaquo;](#example)

### Usage

#### Making a chain

Middleware chains are constructed with [`stack.New()`](http://godoc.org/github.com/alexedwards/stack#New):

```go
stack.New(middlewareOne, middlewareTwo, middlewareThree)
```

You can also store middleware chains as variables, and then [`Append()`](http://godoc.org/github.com/alexedwards/stack#Chain.Append) to them:

```go
stdStack := stack.New(middlewareOne, middlewareTwo)
extStack := stdStack.Append(middlewareThree, middlewareFour)
```

Your middleware should have the signature `func(*stack.Context, http.Handler) http.Handler`. For example:

```go
func middlewareOne(ctx *stack.Context, next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // do something middleware-ish, accessing ctx
    next.ServeHTTP(w, r)
  })
}
```

You can also use middleware with the signature `func(http.Handler) http.Handler` by adapting it with [`stack.Adapt()`](http://godoc.org/github.com/alexedwards/stack#Adapt). For example, if you had the middleware:

```go
func middlewareTwo(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // do something else middleware-ish
    next.ServeHTTP(w, r)
  })
}
```

You can add it to a chain like this:

```go
stack.New(middlewareOne, stack.Adapt(middlewareTwo), middlewareThree)
```

See the [codes samples](#code-samples) for real-life use of third-party middleware with Stack.

#### Adding an application handler

Application handlers should have the signature `func(*stack.Context, http.ResponseWriter, *http.Request)`. You add them to the end of a middleware chain with the [`Then()`](http://godoc.org/github.com/alexedwards/stack#Chain.Then) method. 

So an application handler like this:

```go
func appHandler(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
   // do something handler-ish, accessing ctx
}
```

Is added to the end of a middleware chain like this:

```go
stack.New(middlewareOne, middlewareTwo).Then(appHandler)
```

For convenience [`ThenHandler()`](http://godoc.org/github.com/alexedwards/stack#Chain.ThenHandler) and [`ThenHandlerFunc()`](http://godoc.org/github.com/alexedwards/stack#Chain.ThenHandlerFunc) methods are also provided. These allow you to finish a chain with a standard `http.Handler` or `http.HandlerFunc` respectively.

For example, you could use a standard `http.FileServer` as the application handler:

```go
fs :=  http.FileServer(http.Dir("./static/"))
http.Handle("/", stack.New(middlewareOne, middlewareTwo).ThenHandler(fs))
```

Once a chain is 'closed' with any of these methods it is converted into a [`HandlerChain`](http://godoc.org/github.com/alexedwards/stack#HandlerChain) object which satisfies the `http.Handler` interface, and can be used with the `http.DefaultServeMux` and many other routers.

#### Using context

Request-scoped data (or *context*) can be passed through the chain by storing it in `stack.Context`. This is implemented as a pointer to a `map[string]interface{}` and scoped to the goroutine executing the current HTTP request. Operations on `stack.Context` are protected by a mutex, so if you need to pass the context pointer to another goroutine (say for logging or completing a background process) it is safe for concurrent use.

Data is added with [`Context.Put()`](http://godoc.org/github.com/alexedwards/stack#Context.Put). The first parameter is a string (which acts as a key) and the second is the value you need to store. For example:

```go
func middlewareOne(ctx *stack.Context, next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    ctx.Put("token", "c9e452805dee5044ba520198628abcaa")
    next.ServeHTTP(w, r)
  })
}
```

You retrieve data with [`Context.Get()`](http://godoc.org/github.com/alexedwards/stack#Context.Get). Remember to type assert the returned value into the type you're expecting.

```go
func appHandler(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
  token, ok := ctx.Get("token").(string)
  if !ok {
    http.Error(w, http.StatusText(500), 500)
    return
  }
  fmt.Fprintf(w, "Token is: %s", token)
}
```

Note that `Context.Get()` will return `nil` if a key does not exist. If you need to tell the difference between a key having a `nil` value and it explicitly not existing, please check with [`Context.Exists()`](http://godoc.org/github.com/alexedwards/stack#Context.Exists).

Keys (and their values) can be deleted with [`Context.Delete()`](http://godoc.org/github.com/alexedwards/stack#Context.Delete).    

#### Injecting context

It's possible to inject values into `stack.Context` during a request cycle but *before* the chain starts to be executed. This is useful if you need to inject parameters from a router into the context.

The [`Inject()`](http://godoc.org/github.com/alexedwards/stack#Inject) function returns a new copy of the chain containing the injected context. You should make sure that you use this new copy &ndash; not the original &ndash; for subsequent processing.

Here's an example of a wrapper for injecting [httprouter](https://github.com/julienschmidt/httprouter) params into the context:

```go
func InjectParams(hc stack.HandlerChain) httprouter.Handle {
  return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
    newHandlerChain := stack.Inject(hc, "params", ps)
    newHandlerChain.ServeHTTP(w, r)
  }
}
```

A full example is available in the [code samples](#code-samples).

### Example

```go
package main

import (
  "net/http"
  "github.com/alexedwards/stack"
  "fmt"
)

func main() {
  stk := stack.New(token, stack.Adapt(language))

  http.Handle("/", stk.Then(final))

  http.ListenAndServe(":3000", nil)
}

func token(ctx *stack.Context, next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    ctx.Put("token", "c9e452805dee5044ba520198628abcaa")
    next.ServeHTTP(w, r)
  })
}

func language(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Language", "en-gb")
    next.ServeHTTP(w, r)
  })
}

func final(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
  token, ok := ctx.Get("token").(string)
  if !ok {
    http.Error(w, http.StatusText(500), 500)
    return
  }
  fmt.Fprintf(w, "Token is: %s", token)
}
```

### Code samples

* [Integrating with httprouter](https://gist.github.com/alexedwards/4d20c505f389597c3360)
* *More to follow*

### TODO 

- Add more code samples (using 3rd party middleware)
- Make a `chain.Merge()` method
- Mirror master in v1 branch (and mention gopkg.in in README)
- Add benchmarks
