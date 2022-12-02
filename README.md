# glimrr
glimrr: GitLab Interactive Merge Request Review

Glimrr is a TUI for conducting gitlab merge request reviews at the terminal. It aims to be lighter and faster than the browser based interface, and hopefully more keyboard ergonomic.


# Dev Notes

To allow glimrr to access and modify merge requests, set the `GLIMRR_TOKEN` environment variable.

To build and run:

```
go build -o glimrr ./src && LOG_LEVEL=DEBUG ./glimrr
```

For an auto-reloading dev setup, run:

```
fswatch --exclude ".*\.sw[px]$" --exclude ".*~$" -o ./src | ./watch.sh
```

Note that you'll need to get [fswatch](https://emcrisostomo.github.io/fswatch/) somehow, likely from your package manager. I used brew.
