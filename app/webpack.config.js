const path = require('path')
const pkg = require('./package.json')

const DEBUG = process.env.NODE_ENV !== 'production'

module.exports = {
    mode: DEBUG ? 'development' : 'production',
    context: path.join(__dirname, './'),
    entry: [
        './src/app.js',
    ],
    target: 'web',
    devtool: DEBUG ? 'inline-source-map' : false,
    output: {
        library: {
            name: 'App',
            type: 'var',
            export: 'default'
        },
        path: path.resolve(pkg.config.buildDir),
        publicPath: DEBUG ? '/' : './',
        filename: DEBUG ? 'app.js' : 'app-[contenthash].js',
        clean: true
    },
    resolve: {
        fallback: {
            fs: false
        }
    },
    module: {
        rules: [
            {
                test: /\.js$/,
                exclude: /node_modules/,
                loader: 'babel-loader',
                options: {
                    plugins: ['@babel/plugin-transform-runtime'],
                    presets: [
                        ['@babel/preset-env', {modules: false}]
                    ]
                }
            },
            {
                test: /\.html$/,
                exclude: /node_modules/,
                type: 'asset/resource',
                generator: {
                    filename: '[path][name][ext]'
                }
            },
            {
                test: /\.jpe?g$|\.svg$|\.png$/,
                exclude: /node_modules/,
                type: 'asset/resource',
                generator: {
                    filename: '[path][name][ext]'
                }
            },
            {
                test: /\.(otf|eot|svg|ttf|woff|woff2)(\?v=\d+\.\d+\.\d+)?$/,
                type: 'asset',
                parser: {
                    dataUrlCondition: {
                        maxSize: 8192
                    }
                },
                generator: {
                    mimetype: 'application/font-woff'
                }
            }
        ]
    }
}
