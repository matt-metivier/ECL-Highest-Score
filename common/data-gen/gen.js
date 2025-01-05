#!/usr/bin/env node
var Chance = require('chance');
var fs = require('fs');
var path = require('path');

const dataDir = process.env.NODE_DATA_DIR || '.';
var chance = new Chance('emeraldcloud_seed');

function generate(count, filename) {
	var out = '';
	var bytes = '';
	const fullPath = path.join(dataDir, filename);
	var handle = fs.openSync(fullPath, 'a');

	for (i=0; i<=count; i++) {
		out = chance.natural()  + ': {"id":"' +  chance.guid() + '","data":"' + chance.paragraph({ sentences: 5 }) + "\"\}";
		bytes = fs.writeSync(handle, out + '\n', null, null);         
	};
	fs.closeSync(handle);
	console.log(filename + " write/append completed\n");
}

generate(100,"example_input_data_1.data");
generate(10000,"example_input_data_2.data");
generate(10000,"example_input_data_3.data");
generate(10000,"example_input_data_3.data");
generate(10000,"example_input_data_3.data");
generate(10000,"example_input_data_3.data");
generate(10000,"example_input_data_3.data");
generate(10000,"example_input_data_3.data");
