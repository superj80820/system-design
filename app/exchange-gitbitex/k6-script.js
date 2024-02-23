import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  discardResponseBodies: true,
  // A number specifying the number of VUs to run concurrently.
  vus: 5000,
  // A string specifying the total duration of the test run.
  duration: '55s',

  // The following section contains configuration options for execution of this
  // test script in Grafana Cloud.
  //
  // See https://grafana.com/docs/grafana-cloud/k6/get-started/run-cloud-tests-from-the-cli/
  // to learn about authoring and running k6 test scripts in Grafana k6 Cloud.
  //
  // ext: {
  //   loadimpact: {
  //     // The ID of the project to which the test is assigned in the k6 Cloud UI.
  //     // By default tests are executed in default project.
  //     projectID: "",
  //     // The name of the test in the k6 Cloud UI.
  //     // Test runs with the same name will be grouped.
  //     name: "script.js"
  //   }
  // },

  // Uncomment this section to enable the use of Browser API in your tests.
  //
  // See https://grafana.com/docs/k6/latest/using-k6-browser/running-browser-tests/ to learn more
  // about using Browser API in your test scripts.
  //
  // scenarios: {
  //   // The scenario name appears in the result summary, tags, and so on.
  //   // You can give the scenario any name, as long as each name in the script is unique.
  //   ui: {
  //     // Executor is a mandatory parameter for browser-based tests.
  //     // Shared iterations in this case tells k6 to reuse VUs to execute iterations.
  //     //
  //     // See https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ for other executor types.
  //     executor: 'shared-iterations',
  //     options: {
  //       browser: {
  //         // This is a mandatory parameter that instructs k6 to launch and
  //         // connect to a chromium-based browser, and use it to run UI-based
  //         // tests.
  //         type: 'chromium',
  //       },
  //     },
  //   },
  // }
};

// The function that defines VU logic.
//
// See https://grafana.com/docs/k6/latest/examples/get-started-with-k6/ to learn more
// about authoring k6 scripts.
//
export default function () {
  const url = 'http://localhost:9090/api/orders';
  const randomVal = Math.floor(Math.random() * 100)
  const direction = ["buy", "sell"][Math.floor(Math.random() * 2)]
  const payload = JSON.stringify({ "productId": "BTC-USDT", "side": direction, "price": 2.34 + randomVal, "size": randomVal, "type": "limit" });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authentication': 'Bearer eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MDg2NDI4MDksImlhdCI6MTcwODYzOTIwOSwic3ViIjoiMTc2MDc1NjY2NTA0MjM0MTg4OCJ9.XqhDQGlg0djpzw1wVjb8AfpLHFtlbiFNR780DVg8r7zKW707lcWV5Nzx3TJYuTdk-ZOJ3TK1Y_TPkeBx6bEAsQ',
    },
  };

  const res = http.post(url, payload, params);
  check(res, { "is 200": res => res.status === 200 })
}

// export default function () {
//   // const url = 'http://15.168.70.185/api/health';
//   const url = 'http://15.168.70.185/api/kafkaWrite';
// //  const url = 'http://15.168.70.185'
//   // const url = 'https://yorkloadtest.s3.ap-northeast-3.amazonaws.com/icon.png?response-content-disposition=inline&X-Amz-Security-Token=IQoJb3JpZ2luX2VjEHUaDmFwLW5vcnRoZWFzdC0yIkcwRQIhAP%2F9pghiU2WD2ufG4bFs4K1LLx3XqX3eAqcwIMDWHAyQAiAGaM5Wdsu16uyVLbQoJAHSOtg767XKp4eiC4ct3%2BfFkyrkAgheEAAaDDI1NzA4OTgzODQ4MSIMWxLfcupbjfB%2FZVhYKsECehd9NTCqNIgxoDNCbP2fT3OJWOVo8%2BjCUc2VII70Wc%2Bqeu%2Fdp5uSIs7vC64galWqyFGuK%2BKgrTdRul%2FaxwjPQ0pAiY4wopB1LC8UcrOIKLJM1sJDonxl2r%2FULrnQCJssTsNnh8oUlm0xYd5ToMXY1FmuZaXODebqPOxzo7ZTTkRDaSwoIFfXy6cWuLwFRpAsEkspMyJuOvQEFPIm4yb7mCI8bfWS%2BIU3NgvVRbpCw1feD2iuwq1uaUUey7O3dceGcMX9GnT7z5eBIQJkmECS3PrzfvnNxlSeVlLrt7OMLjQX%2BStUYtoGp3zsXcE%2FmkKCT%2FY8Vg8ckP4tvd4pbyd9ckHEahbAN3EYLZ1fGrPI1LObmJn%2FViPIVscY9arvXGJfG8HHer37xtAJyjDybE03UZcUU52vU%2BGDEjsuvJsCcbflMMSE3K4GOrMCMbUYn%2BiAStzTH8frh%2FEdRvTE55687ZhsB%2B3EKtESoOJ%2BymlfgnaqKW33dHcDmfJoY0t3hLDfnuAe1PUgY65vd2AHK49S6GYluAAq5BIbF2MzZrOxREoNTGM%2BHUSH6N4i3c%2FSKihoonwd3B4FeSdkKHudDJqIqjnDmqWMyetqJdP0up2lHis9tIMdBVBvHCMt%2B5glPdwa4Kl%2FEF66Qj3pwPjpPpMSatIbB4aYLVVR8pXGtFHRdHP2obcuth477RaMpJfkACmoQUlrpMbIT2Ag61ekTYmtoP6WGmDwCtczZYhiK1kGVVi659bEkyDOfQ%2FAbhhdEAhRtGRmSrnb4ENaVBjEUCpg4hH2rDFCr2JTz3DwuwCygtkOZKbnTqqMv4QCSH%2BftZst15NfHI4IznBtSu%2B1VA%3D%3D&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Date=20240222T132220Z&X-Amz-SignedHeaders=host&X-Amz-Expires=43200&X-Amz-Credential=ASIATXW57TWI2WYQTPYU%2F20240222%2Fap-northeast-3%2Fs3%2Faws4_request&X-Amz-Signature=f8802a54754fa701898260dea4bdc108a98494be004359da539b2905df8396b7'
//   const res = http.get(url);
// //  sleep(0.1);
// }

// set datafile separator ','
// set terminal pngcairo background rgb 'white' linewidth 4 size 1200,700 enhanced font 'Arial,16'
// set bmargin at screen 130.0/600
// set rmargin at screen 1080.0/1200
// set output sprintf('%s-%s.png', ec2instance, script)

// set title sprintf('k6 / EC2 %s / %s', ec2instance, script) font 'Arial Bold,20'
// set key at graph 0.6, 0.3 autotitle columnhead

// set border 31 lw 0.5
// set style data lines
// set style line 100 lt 1 lc rgb "grey" lw 0.5 # linestyle for the grid
// set grid ls 100

// set xtics 5
// set xlabel 'Time (M:S)'
// set xdata time
// set format x '%M:%S'
// set xtics rotate
// set y2tics           # enable second axis
// set ytics nomirror   # dont show the tics on that side
// set y2range [0:]     # start from 0
// set y2label "CPU (%)"

// plot sprintf('%s-%s.csv', ec2instance, script) using ($1):2 axis x1y2 lc rgb '#00d8bfd8', \
//      '' using ($1):($3 / 1000) title 'RAM (MB)', \
//      '' using ($1):4, \
//      '' using ($1):5