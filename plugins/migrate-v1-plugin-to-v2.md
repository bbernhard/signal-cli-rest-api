Migrating a plugin from version `1` to version `2` is really easy. 

* Change your plugin definition (`*.def`) file 
  and set the version to `2` 
e.g:

```
endpoint: my-custom-send-endpoint/:number 
method: POST
version: 2
```

* Change your plugin script
  and implement the `exec` (and optionally the `init`) functions.

e.g:

```
function exec()
    -- your plugin code goes here
end 

function init()
    -- if your script needs some additional setup (e.g a sqlite database, a config file, etc)
    -- the initialization can be done here.
end
```


