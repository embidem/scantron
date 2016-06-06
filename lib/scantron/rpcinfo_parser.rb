module Scantron
  class RPCInfoParser
    def parse(output, port_number)
      lines = output.split("\n")
      without_header = lines.drop(1)

      line = without_header.
        map { |l| l.split(" ") }.
        find { |components| components[3] == port_number.to_s }

      return nil if line.nil?

      line[4]
    end
  end
end
