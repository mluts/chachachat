# chachachat
The attempt to do a chat in go

# How to use the server
go run chat.go main.go :9999

# How to use the client
terminal1:
go run client/main.go localhost:9999 user1

terminal2:
go run client/main.go localhost:9999 user2

See your messages appear on the other side when you write something

# What can be better
The connections/users lookup is O(n), but can be O(1)
