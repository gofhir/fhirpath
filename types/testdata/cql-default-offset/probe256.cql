library Differential version '1.0'
using FHIR version '4.0.1'
include FHIRHelpers version '4.0.1'
define D0: @2012-01-01T < @2012-01-01T05:00:00.000Z
define D1: @2012-01-01T = @2012-01-01T01:00:00.000Z
define D2: @2012-01-01T = @2012-01-02T02:00:00.000Z
define D3: @2012-01-01T > @2011-12-31T23:00:00.000Z
define D4: @2012-01T = @2012-02-01T02:00:00.000Z
define D5: @2012T = @2013-01-01T02:00:00.000Z
define D6: @2012-01-01T = @2012-01-01T
define D7: @2012-01-01T = @2012-01-01T10:00:00.000
define D8: @2020-01-01T10+05:30 < @2020-01-01T04:45Z
define D9: @2020-01-01T10:00+05:30 < @2020-01-01T04:45Z
define D10: @2020-01-01T10:00+05:30 = @2020-01-01T05:15Z
define D11: days between @2012-01-01T and @2012-01-03T02:00:00.000Z
define D12: hours between @2012-01-01T and @2012-01-02T05:00:00.000Z
define D13: @2012-01-01T same day as @2012-01-01T01:00:00.000Z
define D14: ToDateTime(@2012-01-01) < @2012-01-01T05:00:00.000Z
define D15: @2016-12-31T = @2017-01-01T02:59:59.999Z
define D16: @2012T < @2013-01-01T00:00:00.000Z
define D17: @2012-12-31T < @2013-01-01T10:00:00.000Z
define D18: @2012-01-01T = DateTime(2012, 1, 1, 0, 0, 0, 0, 14.0)
define D19: @2012-01-01T = DateTime(2012, 1, 1, 23, 0, 0, 0, -14.0)
define D20: days between @2012-01-01T and @2012-01-03T02:00:00.000Z
define D21: weeks between @2012-01-01T and @2012-01-08T02:00:00.000Z
