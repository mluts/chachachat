# chachachat
The attempt to do a chat in go

# How to use the server
```
go install github.com/mluts/chachachat
~/go/bin/chachachat :9999
```

# How to use the client
```
go install github.com/mluts/chachachat/chachachat-client
terminal1:
~/go/bin/chachachat-client :9999 user1

terminal2:
~/go/bin/chachachat-client :9999 user2
```

See your messages appear on the other side when you write something

# What can be better
The connections/users lookup is O(n), but can be O(1)
