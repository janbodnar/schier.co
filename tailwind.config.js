const { colors } = require('tailwindcss/defaultTheme');

module.exports = {
  important: true,
  theme: {
    colors: {
      primary: colors.blue,
      background: colors.white,
      foreground: colors.gray['800'],
      gray: colors.gray,
    },
    textColor: {
      inverted: colors.gray['200'],
      primary: colors.gray['700'],
      secondary: colors.blue['500'],
      danger: colors.red['500'],
      gray: colors.gray,
      syntax1: colors.purple['300'],
      syntax2: colors.blue['300'],
      syntax3: colors.indigo['300'],
    },
    opacity: {
      '0': '0',
      '10': '.1',
      '20': '.2',
      '30': '.3',
      '40': '.4',
      '50': '.5',
      '60': '.6',
      '70': '.7',
      '80': '.8',
      '90': '.9',
      '100': '1',
    },
  },
};
