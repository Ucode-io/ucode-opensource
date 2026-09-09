var {Parser} = require("hot-formula-parser");

let parser = new Parser()

function jsFunction(value) {

    const {error, result} = parser.parse(value)
    newValue = error ?? result
     return newValue
}

const input = process.argv[2];
const output = jsFunction(input);


console.log(output);