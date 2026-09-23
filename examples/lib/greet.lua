local greet = {}

function greet.hello(name)
  return string.format("hello, %s!", name or "world")
end

return greet
