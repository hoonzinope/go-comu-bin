# go-comu-bin

single binary community engine for go

## structure

delivery -> useCaseInterface -> application Layer -> repository Layer -> infrastructure

## domain
- user
    - id
    - name
    - password
    - role (admin or user)
    - createdAt

- board
    - id
    - name
    - description
    - createdAt

- post
    - id
    - title
    - content
    - authorId
    - boardId
    - createdAt
    - updatedAt

- comment
    - id
    - content
    - authorId
    - postId
    - createdAt
    - updatedAt

- reaction
    - id
    - targetType (post or comment)
    - targetId (postId or commentId)
    - userId
    - type (like or dislike)
    - createdAt

## useCase
- user
    - signUp
    - signOut
    - login
    - logout
    * config admin user (when binary startup)

- board
    - createBoard (only admin)
    - deleteBoard (only admin)
    - updateBoard (only admin)
    - getBoard(s)

- post
    - createPost (only registered user)
    - deletePost (only author or admin)
    - updatePost (only author or admin)
    - getPost(s)

- comment
    - createComment (only registered user)
    - deleteComment (only author or admin)
    - updateComment (only author or admin)
    - getComment(s)

- reaction
    - addReaction (only registered user)
    - removeReaction (only author or admin)
    - getReactions(targetID, targetType)